/* * Copyright (c) 2017, MegaEase
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

package statussynccontroller

import (
	"runtime/debug"
	"sync"

	"gopkg.in/yaml.v2"

	"github.com/megaease/easegress/pkg/cluster"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/util/timetool"

	"github.com/megaease/easegress/pkg/object/trafficcontroller"
)

const (
	// Category is the category of StatusSyncController.
	Category = supervisor.CategorySystemController

	// Kind is the kind of StatusSyncController.
	Kind = "StatusSyncController"

	maxStatusesRecordCount = 10
)

type (
	// StatusSyncController is a system controller to synchronize
	// status of every object to remote storage.
	StatusSyncController struct {
		superSpec *supervisor.Spec
		spec      *Spec

		timer *timetool.DistributedTimer

		lastSyncStatusUnits map[string]*statusUnit

		// sorted by timestamp in ascending order
		statusSnapshots      []*StatusesSnapshot
		statusSnapshotsMutex sync.RWMutex

		// statusUpdateMaxBatchSize is maximum statuses to update in one cluster transaction
		statusUpdateMaxBatchSize int

		done chan struct{}
	}

	// Spec describes StatusSyncController.
	Spec struct{}

	// StatusesSnapshot is the history record for status of every running object.
	StatusesSnapshot struct {
		Statuses      map[string]*supervisor.Status
		UnixTimestamp int64
	}

	statusUnit struct {
		namespace  string
		objectName string
		timestamp  int64
		status     interface{}
	}
)

func newStatusUnit(namespace, objectName string, timestamp int64, status interface{}) *statusUnit {
	return &statusUnit{
		namespace:  namespace,
		objectName: objectName,
		timestamp:  timestamp,
		status:     status,
	}
}

func (s *statusUnit) clusterKey(layout *cluster.Layout) string {
	return layout.StatusObjectKey(s.namespace, s.objectName)
}

func (s *statusUnit) id() string {
	return s.namespace + "/" + s.objectName
}

func (s *statusUnit) marshal() ([]byte, error) {
	buff, err := yaml.Marshal(s.status)
	if err != nil {
		return nil, err
	}

	m := map[string]interface{}{}
	err = yaml.Unmarshal(buff, &m)
	if err != nil {
		return nil, err
	}

	if m == nil {
		m = map[string]interface{}{}
	}

	m["timestamp"] = s.timestamp

	buff, err = yaml.Marshal(m)
	if err != nil {
		return nil, err
	}

	return buff, nil
}

func init() {
	supervisor.Register(&StatusSyncController{})
}

// Category returns the category of StatusSyncController.
func (ssc *StatusSyncController) Category() supervisor.ObjectCategory {
	return Category
}

// Kind return the kind of StatusSyncController.
func (ssc *StatusSyncController) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of StatusSyncController.
func (ssc *StatusSyncController) DefaultSpec() interface{} {
	return &Spec{}
}

// Init initializes StatusSyncController.
func (ssc *StatusSyncController) Init(superSpec *supervisor.Spec) {
	ssc.superSpec, ssc.spec = superSpec, superSpec.ObjectSpec().(*Spec)
	ssc.reload()
}

// Inherit inherits previous generation of StatusSyncController.
func (ssc *StatusSyncController) Inherit(spec *supervisor.Spec, previousGeneration supervisor.Object) {
	previousGeneration.Close()
	ssc.Init(spec)
}

func (ssc *StatusSyncController) reload() {
	ssc.timer = timetool.NewDistributedTimer(nextSyncStatusDuration)
	ssc.done = make(chan struct{})

	opts := ssc.superSpec.Super().Options()
	ssc.statusUpdateMaxBatchSize = opts.StatusUpdateMaxBatchSize
	if ssc.statusUpdateMaxBatchSize < 1 {
		ssc.statusUpdateMaxBatchSize = 20
	}
	logger.Infof("StatusUpdateMaxBatchSize is %d", ssc.statusUpdateMaxBatchSize)

	go ssc.run()
}

func (ssc *StatusSyncController) run() {
	for {
		select {
		case t := <-ssc.timer.C:
			ssc.handleStatus(t.Unix())
		case <-ssc.done:
			return
		}
	}
}

// Status returns the status of StatusSyncController.
func (ssc *StatusSyncController) Status() *supervisor.Status {
	return &supervisor.Status{
		ObjectStatus: struct{}{},
	}
}

// Close closes StatusSyncController.
func (ssc *StatusSyncController) Close() {
	close(ssc.done)
	ssc.timer.Close()
}

func (ssc *StatusSyncController) handleStatus(unixTimestamp int64) {
	statusUnits := make(map[string]*statusUnit)

	walkFn := func(entity *supervisor.ObjectEntity) bool {
		defer func() {
			if err := recover(); err != nil {
				logger.Errorf("recover from syncStatus, err: %v, stack trace:\n%s\n",
					err, debug.Stack())
			}
		}()

		namespace := cluster.NamespaceDefault
		objectName := entity.Spec().Name()
		status := entity.Instance().Status()
		status.Timestamp = unixTimestamp

		switch objectStatus := status.ObjectStatus.(type) {
		case *trafficcontroller.Status:
			for _, namespaceStatus := range objectStatus.Namespaces {
				namespace := namespaceStatus.TrafficNamespace()
				for _, trafficObject := range namespaceStatus.TrafficObjects {
					su := newStatusUnit(namespace, trafficObject.Name,
						unixTimestamp, trafficObject.TrafficObjectStatus)
					statusUnits[su.id()] = su
				}
			}
		default:
			su := newStatusUnit(namespace, objectName, unixTimestamp, status.ObjectStatus)
			statusUnits[su.id()] = su
		}

		return true
	}

	ssc.superSpec.Super().WalkControllers(walkFn)

	ssc.takeSnapshot(statusUnits, unixTimestamp)
	ssc.syncStatusToCluster(statusUnits)
}

func (ssc *StatusSyncController) syncStatusToCluster(statusUnits map[string]*statusUnit) {
	// Delete statuses which disappeared in current status.
	if ssc.lastSyncStatusUnits != nil {
		kvs := make(map[string]*string)
		for k := range ssc.lastSyncStatusUnits {
			if su, exists := statusUnits[k]; !exists {
				k = su.clusterKey(ssc.superSpec.Super().Cluster().Layout())
				kvs[k] = nil
			}
		}
		err := ssc.superSpec.Super().Cluster().PutAndDeleteUnderLease(kvs)
		if err != nil {
			logger.Errorf("sync status failed. If the message size is too large, "+
				"please increase the value of cluster.MaxCallSendMsgSize in configuration: %v", err)
		}
	}

	ssc.lastSyncStatusUnits = statusUnits

	kvs := make(map[string]*string)
	for _, su := range statusUnits {
		key := su.clusterKey(ssc.superSpec.Super().Cluster().Layout())
		buff, err := su.marshal()
		if err != nil {
			logger.Errorf("BUG: marshal %#v failed: %v", su, err)
			continue
		}

		value := string(buff)
		kvs[key] = &value

		if len(kvs) >= ssc.statusUpdateMaxBatchSize {
			err := ssc.superSpec.Super().Cluster().PutAndDeleteUnderLease(kvs)
			if err != nil {
				logger.Errorf("sync status failed. If the message size is too large, "+
					"please increase the value of cluster.MaxCallSendMsgSize in configuration: %v", err)
			}
			kvs = make(map[string]*string)
		}
	}

	if len(kvs) > 0 {
		err := ssc.superSpec.Super().Cluster().PutAndDeleteUnderLease(kvs)
		if err != nil {
			logger.Errorf("sync status failed. If the message size is too large, "+
				"please increase the value of cluster.MaxCallSendMsgSize in configuration: %v", err)
		}
	}
}

func (ssc *StatusSyncController) takeSnapshot(statusUnits map[string]*statusUnit, timestamp int64) {
	snapshot := &StatusesSnapshot{
		Statuses:      make(map[string]*supervisor.Status),
		UnixTimestamp: timestamp,
	}

	for _, su := range statusUnits {
		snapshot.Statuses[su.id()] = &supervisor.Status{
			ObjectStatus: su.status,
			Timestamp:    timestamp,
		}
	}

	ssc.statusSnapshotsMutex.Lock()
	defer ssc.statusSnapshotsMutex.Unlock()

	ssc.statusSnapshots = append(ssc.statusSnapshots, snapshot)
	if len(ssc.statusSnapshots) > maxStatusesRecordCount {
		ssc.statusSnapshots = ssc.statusSnapshots[1:]
	}
}

// GetStatusSnapshots return the latest status snapshots.
func (ssc *StatusSyncController) GetStatusSnapshots() []*StatusesSnapshot {
	ssc.statusSnapshotsMutex.RLock()
	defer ssc.statusSnapshotsMutex.RUnlock()

	records := make([]*StatusesSnapshot, len(ssc.statusSnapshots))
	copy(records, ssc.statusSnapshots)
	return records
}
