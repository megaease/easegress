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

package spec

import (
	"fmt"
	"testing"
)

func TestValidFSM(t *testing.T) {
	fsm, err := InitFSM(InactiveState)
	if err != nil {
		t.Errorf("test failed spec should not be valid, err: %v", err)
	}
	err = fsm.Next(ErrorEvent)
	if err != nil {
		t.Errorf("next failed: %v", err)
	}

	fmt.Printf("FSM SUCC, current event is %s\n", fsm.currentState)
}

func TestInValidStateFSM(t *testing.T) {
	_, err := InitFSM("unknown")
	if err == nil {
		t.Errorf("init fsm should be failed")
	}
}

func TestInValidEventFSM(t *testing.T) {
	fsm, err := InitFSM(ActiveState)
	if err != nil {
		t.Errorf("init fsm should be succ, err: %v", err)
	}
	err = fsm.Next(UpdateEvent)
	if err == nil {
		t.Errorf("update should not be allowed at active state")
	}
}

func TestUnknownEventFSM(t *testing.T) {
	fsm, err := InitFSM(ActiveState)
	if err != nil {
		t.Errorf("init fsm should be succ, err: %v", err)
	}
	err = fsm.Next("unknown")
	if err == nil {
		t.Errorf("unknown event should not be allowed!")
	}
}

func TestValidEventFSM(t *testing.T) {
	fsm, err := InitFSM(ActiveState)
	if err != nil {
		t.Errorf("init fsm should be succ, err: %v", err)
	}
	err = fsm.Next(ReadyEvent)
	if err != nil {
		t.Errorf("provision ok should be allowed in active state")
	}
}

func TestValidEventAtPending1(t *testing.T) {
	fsm, err := InitFSM(InactiveState)
	if err != nil {
		t.Errorf("init fsm should be succ, err: %v", err)
	}
	err = fsm.Next(ReadyEvent)
	if err != nil {
		t.Errorf("provision ok should be allowed in active state")
	}
	if fsm.currentState != ActiveState {
		t.Errorf("pending's next state should be active after provision ok event!")
	}
}

func TestValidEventAtPending2(t *testing.T) {
	fsm, err := InitFSM(InactiveState)
	if err != nil {
		t.Errorf("init fsm should be succ, err: %v", err)
	}
	err = fsm.Next(ErrorEvent)
	if err != nil {
		t.Errorf("provision failed should be allowed in pending state")
	}

	if fsm.currentState != FailedState {
		t.Errorf("pending's next state should be failed after provision failed event!")
	}
}

func TestValidEventAtPending3(t *testing.T) {
	fsm, err := InitFSM(InactiveState)
	if err != nil {
		t.Errorf("init fsm should be succ, err: %v", err)
	}
	err = fsm.Next(PendingEvent)
	if err != nil {
		t.Errorf("Pending event should be allowed in inactive state")
	}

	if fsm.currentState != FailedState {
		t.Errorf("inactive's next state should be failed after pending event!")
	}
}

func TestValidEventAtPending4(t *testing.T) {
	fsm, err := InitFSM(InactiveState)
	if err != nil {
		t.Errorf("init fsm should be succ, err: %v", err)
	}
	err = fsm.Next(UpdateEvent)
	if err != nil {
		t.Errorf("update should be allowed in pending state")
	}

	if fsm.currentState != InitialState {
		t.Errorf("pending's next state should be pending after update event!")
	}
}

func TestValidEventAtPending5(t *testing.T) {
	fsm, err := InitFSM(InactiveState)
	if err != nil {
		t.Errorf("init fsm should be succ, err: %v", err)
	}
	err = fsm.Next(DeleteEvent)
	if err != nil {
		t.Errorf("delete should be allowed in pending state")
	}

	if fsm.currentState != DestroyedState {
		t.Errorf("pending's next state should be removed after delete event!")
	}
}

func TestValidEventAtActive1(t *testing.T) {
	fsm, err := InitFSM(ActiveState)
	if err != nil {
		t.Errorf("init fsm should be succ, err: %v", err)
	}
	err = fsm.Next(ErrorEvent)
	if err != nil {
		t.Errorf("provision failed event should be allowed in active state")
	}

	if fsm.currentState != FailedState {
		t.Errorf("active's next state should be failed after provision failed!")
	}
}

func TestValidEventAtActive2(t *testing.T) {
	fsm, err := InitFSM(ActiveState)
	if err != nil {
		t.Errorf("init fsm should be succ, err: %v", err)
	}
	err = fsm.Next(ReadyEvent)
	if err != nil {
		t.Errorf("provision ok event should be allowed in active state")
	}

	if fsm.currentState != ActiveState {
		t.Errorf("active's next state should be active after provision ok!")
	}
}

func TestValidEventAtActive3(t *testing.T) {
	fsm, err := InitFSM(ActiveState)
	if err != nil {
		t.Errorf("init fsm should be succ, err: %v", err)
	}
	err = fsm.Next(PendingEvent)
	if err != nil {
		t.Errorf("provision pending event should be allowed in active state")
	}

	if fsm.currentState != FailedState {
		t.Errorf("active's next state should be active after provision pending!")
	}
}

func TestValidEventAtActive4(t *testing.T) {
	fsm, err := InitFSM(ActiveState)
	if err != nil {
		t.Errorf("init fsm should be succ, err: %v", err)
	}
	err = fsm.Next(StopEvent)
	if err != nil {
		t.Errorf("stop event should be allowed in active state")
	}

	if fsm.currentState != InactiveState {
		t.Errorf("active's next state should be inactive after stoping!")
	}
}

func TestInValidEventAtActive(t *testing.T) {
	fsm, err := InitFSM(ActiveState)
	if err != nil {
		t.Errorf("init fsm should be succ, err: %v", err)
	}
	err = fsm.Next(StartEvent)
	if err == nil {
		t.Errorf("start event should not be allowed in active state")
	}
}

func TestValidEventAtInActive1(t *testing.T) {
	fsm, err := InitFSM(InactiveState)
	if err != nil {
		t.Errorf("init fsm should be succ, err: %v", err)
	}
	err = fsm.Next(ErrorEvent)
	if err != nil {
		t.Errorf("provision failed event should be allowed in inactive state")
	}

	if fsm.currentState != FailedState {
		t.Errorf("Inactive's next state should be failed after provision failed!")
	}
}

func TestValidEventAtInActive2(t *testing.T) {
	fsm, err := InitFSM(InactiveState)
	if err != nil {
		t.Errorf("init fsm should be succ, err: %v", err)
	}
	err = fsm.Next(ReadyEvent)
	if err != nil {
		t.Errorf("provision ok event should be allowed in inactive state")
	}

	if fsm.currentState != ActiveState {
		t.Errorf("inactive's next state should be active after provision ok!")
	}
}

func TestValidEventAtInActive3(t *testing.T) {
	fsm, err := InitFSM(InactiveState)
	if err != nil {
		t.Errorf("init fsm should be succ, err: %v", err)
	}
	err = fsm.Next(PendingEvent)
	if err != nil {
		t.Errorf("provision pending event should be allowed in active state")
	}

	if fsm.currentState != FailedState {
		t.Errorf("active's next state should be pending after provision pending!")
	}
}

func TestValidEventAtInActive4(t *testing.T) {
	fsm, err := InitFSM(InactiveState)
	if err != nil {
		t.Errorf("init fsm should be succ, err: %v", err)
	}
	err = fsm.Next(StartEvent)
	if err != nil {
		t.Errorf("stop event should be allowed in inactive state")
	}

	if fsm.currentState != InactiveState {
		t.Errorf("inactive's next state should be inactive after starting!")
	}
}

func TestValidEventAtFailed1(t *testing.T) {
	fsm, err := InitFSM(FailedState)
	if err != nil {
		t.Errorf("init fsm should be succ, err: %v", err)
	}
	err = fsm.Next(ErrorEvent)
	if err != nil {
		t.Errorf("provision failed event should be allowed in failed state")
	}

	if fsm.currentState != FailedState {
		t.Errorf("failed's next state should be failed after provision failed!")
	}
}

func TestValidEventAtFailed2(t *testing.T) {
	fsm, err := InitFSM(FailedState)
	if err != nil {
		t.Errorf("init fsm should be succ, err: %v", err)
	}
	err = fsm.Next(ReadyEvent)
	if err != nil {
		t.Errorf("provision ok event should be allowed in failed state")
	}

	if fsm.currentState != InitialState {
		t.Errorf("failed's next state should be pending after provision ok!")
	}
}

func TestValidEventAtFailed3(t *testing.T) {
	fsm, err := InitFSM(FailedState)
	if err != nil {
		t.Errorf("init fsm should be succ, err: %v", err)
	}
	err = fsm.Next(PendingEvent)
	if err != nil {
		t.Errorf("provision pending event should be allowed in active state")
	}

	if fsm.currentState != FailedState {
		t.Errorf("failed's next state should be pending after provision pending!")
	}
}

func TestValidEventAtFailed4(t *testing.T) {
	fsm, err := InitFSM(FailedState)
	if err != nil {
		t.Errorf("init fsm should be succ, err: %v", err)
	}
	err = fsm.Next(UpdateEvent)
	if err != nil {
		t.Errorf("update event should be allowed in inactive state")
	}

	if fsm.currentState != InitialState {
		t.Errorf("failed's next state should be pending after updating!")
	}
}
