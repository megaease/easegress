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
)

type (
	// functionFSM is a finite state machine for managing faas function.
	FSM struct {
		currentState State
	}

	// Event is the event type generated by CLI or FaaSProvider.
	Event string

	// State is the FaaSFunction's state.
	State string

	// transition builds a role for state changing
	transition struct {
		From  State
		Event Event
		To    State
	}
)

const (
	// State value of FaaSFunction
	FailedState   State = "failed"
	InitialState  State = "initial"
	ActiveState   State = "active"
	InactiveState State = "inactive"

	// only for keep fsm working
	DesctroyState State = "desctory"

	// Function event invoked by APIs.
	CreateEvent Event = "create"
	StartEvent  Event = "start"
	StopEvent   Event = "stop"
	UpdateEvent Event = "update"
	DeleteEvent Event = "delete"

	// Function Event invoked by FaaSProvider
	ReadyEvent   Event = "ready"
	PendingEvent Event = "pending"
	ErrorEvent   Event = "error"
)

var (
	validState map[State]struct{} = map[State]struct{}{
		InitialState:  {},
		ActiveState:   {},
		InactiveState: {},
		FailedState:   {},
		DesctroyState: {},
	}

	validEvent map[Event]struct{} = map[Event]struct{}{
		UpdateEvent:  {},
		DeleteEvent:  {},
		StopEvent:    {},
		StartEvent:   {},
		CreateEvent:  {},
		PendingEvent: {},
		ErrorEvent:   {},
		ReadyEvent:   {},
	}

	transitions = map[Event][]transition{}
)

func init() {
	table := []transition{
		{InitialState, UpdateEvent, InitialState},
		{InitialState, DeleteEvent, DesctroyState},
		{InitialState, ReadyEvent, ActiveState},
		{InitialState, PendingEvent, InitialState},
		{InitialState, ErrorEvent, FailedState},

		{ActiveState, StopEvent, InactiveState},
		{ActiveState, ErrorEvent, FailedState},
		{ActiveState, ReadyEvent, ActiveState},
		{ActiveState, PendingEvent, FailedState},

		{InactiveState, UpdateEvent, InitialState},
		{InactiveState, StartEvent, InactiveState},
		{InactiveState, DeleteEvent, DesctroyState},
		{InactiveState, ReadyEvent, ActiveState},
		{InactiveState, PendingEvent, FailedState},
		{InactiveState, ErrorEvent, FailedState},

		{FailedState, DeleteEvent, DesctroyState},
		{FailedState, UpdateEvent, InitialState},
		{FailedState, ReadyEvent, InitialState},
		{FailedState, ErrorEvent, FailedState},
		{FailedState, PendingEvent, FailedState},
	}

	// using Event as the key
	for _, t := range table {
		transitions[t.Event] = append(transitions[t.Event], t)
	}
}

// InitState returns the initial FSM state which is the `pending` state.
func InitState() State {
	return InitialState
}

// InitFSM creates a finite state machine by given states
func InitFSM(state State) (*FSM, error) {
	if _, exist := validState[state]; !exist {
		return nil, fmt.Errorf("invalid state: %s", state)
	}
	return &FSM{
		currentState: state,
	}, nil
}

// Next turns the function status into properate state by given event.
func (fsm *FSM) Next(event Event) error {
	if _, exist := validEvent[event]; !exist {
		return fmt.Errorf("unknown event: %s", event)
	}

	if t, exist := transitions[event]; exist {
		for _, v := range t {
			if fsm.currentState == v.From {
				fsm.currentState = v.To
				return nil
			}
		}
	}
	return fmt.Errorf("invalid event: %s, currentState: %s", event, fsm.currentState)
}

// Current gets FSM current state.
func (fsm *FSM) Current() State {
	return fsm.currentState
}
