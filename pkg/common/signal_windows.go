//go:build windows
// +build windows

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

package common

import (
	"fmt"
	"log"
	"os"

	"golang.org/x/sys/windows"
)

func eventName(s Signal, pid int) string {
	return fmt.Sprintf("Global\\easegress_%v_%v", s, pid)
}

// NotifySignal is the windows impl of os/signal.Notify
// which takes abstract Signal and causes common to relay incoming signals to c
//
// On Windows, the impl is based on https://docs.microsoft.com/en-us/windows/win32/api/synchapi/nf-synchapi-createeventw
func NotifySignal(c chan<- Signal, sig ...Signal) error {
	if c == nil {
		return fmt.Errorf("NotifySignal using nil channel")
	}

	pid := os.Getpid()
	evts := make([]windows.Handle, 0, len(sig))

	for _, s := range sig {
		name, err := windows.UTF16PtrFromString(eventName(s, pid))
		if err != nil {
			return err
		}

		h, err := windows.CreateEvent(nil, 1, 0, name)
		if err != nil {
			return err
		}

		evts = append(evts, h)
	}

	go func() {
		for {
			ev, err := windows.WaitForMultipleObjects(evts, false, windows.INFINITE)
			if err != nil {
				log.Printf("WaitForMultipleObjects failed: %v", err)
				return
			}

			offset := ev - windows.WAIT_OBJECT_0
			c <- sig[offset]
			if err := windows.ResetEvent(evts[offset]); err != nil {
				log.Printf("ResetEvent failed: %v", err)
			}
		}
	}()

	return nil
}

// RaiseSignal is the windows impl of syscall.Kill
// any chan passed to NotifySignal will receive the sig
//
// On Windows, the impl is based on https://docs.microsoft.com/en-us/windows/win32/api/synchapi/nf-synchapi-createeventw
func RaiseSignal(pid int, sig Signal) error {
	name, err := windows.UTF16PtrFromString(eventName(sig, pid))
	if err != nil {
		return err
	}

	ev, err := windows.OpenEvent(windows.EVENT_MODIFY_STATE, false, name)
	if err != nil {
		return err
	}

	defer windows.CloseHandle(ev)
	return windows.SetEvent(ev)
}
