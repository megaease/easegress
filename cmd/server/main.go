/*
 * Copyright (c) 2017, MegaEase
 * All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"log"
	"os"
	"sync"

	"github.com/megaease/easegress/pkg/api"
	"github.com/megaease/easegress/pkg/cluster"
	"github.com/megaease/easegress/pkg/common"
	"github.com/megaease/easegress/pkg/env"
	"github.com/megaease/easegress/pkg/graceupdate"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/option"
	"github.com/megaease/easegress/pkg/pidfile"
	"github.com/megaease/easegress/pkg/profile"
	_ "github.com/megaease/easegress/pkg/registry"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/version"
)

func main() {
	opt := option.New()
	msg, err := opt.Parse()
	if err != nil {
		common.Exit(1, err.Error())
	}
	if msg != "" {
		common.Exit(0, msg)
	}

	err = env.InitServerDir(opt)
	if err != nil {
		log.Printf("failed to init env: %v", err)
		os.Exit(1)
	}

	logger.Init(opt)
	defer logger.Sync()
	logger.Infof("%s", version.Long)

	if opt.SignalUpgrade {
		pid, err := pidfile.Read(opt)
		if err != nil {
			logger.Errorf("failed to read pidfile: %v", err)
			os.Exit(1)
		}

		if err := common.RaiseSignal(pid, common.SignalUsr2); err != nil {
			logger.Errorf("failed to send signal: %v", err)
			os.Exit(1)
		}

		logger.Infof("graceful upgrade signal sent")

		return
	}

	// disable force-new-cluster for graceful update
	if graceupdate.IsInherit() {
		opt.ForceNewCluster = false
	} else {
		err := pidfile.Write(opt)
		if err != nil {
			logger.Errorf("write pidfile failed: %v", err)
			os.Exit(1)
		}
	}

	profile, err := profile.New(opt)
	if err != nil {
		logger.Errorf("new profile failed: %v", err)
		os.Exit(1)
	}
	cls, err := cluster.New(opt)
	if err != nil {
		logger.Errorf("new cluster failed: %v", err)
		os.Exit(1)
	}

	super := supervisor.MustNew(opt, cls)

	apiServer := api.MustNewServer(opt, cls, super, profile)

	if graceupdate.CallOriProcessTerm(super.FirstHandleDone()) {
		pidfile.Write(opt)
	}

	closeCls := func() {
		wg := &sync.WaitGroup{}
		wg.Add(2)
		apiServer.Close(wg)
		cls.CloseServer(wg)
		wg.Wait()
	}
	restartCls := func() {
		cls.StartServer()
		apiServer = api.MustNewServer(opt, cls, super, profile)
	}
	if err := graceupdate.NotifySigUsr2(closeCls, restartCls); err != nil {
		log.Printf("failed to notify signal: %v", err)
		os.Exit(1)
	}

	sigChan := make(chan common.Signal, 1)
	if err := common.NotifySignal(sigChan, common.SignalInt, common.SignalTerm); err != nil {
		log.Printf("failed to register signal: %v", err)
		os.Exit(1)
	}
	sig := <-sigChan
	go func() {
		sig := <-sigChan
		logger.Infof("%s signal received, closing easegress immediately", sig)
		os.Exit(255)
	}()
	logger.Infof("%s signal received, closing easegress", sig)

	wg := &sync.WaitGroup{}
	wg.Add(4)
	apiServer.Close(wg)
	super.Close(wg)
	cls.Close(wg)
	profile.Close(wg)
	wg.Wait()
}
