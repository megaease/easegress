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

package cluster

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLayout(t *testing.T) {
	assert := assert.New(t)
	l := Layout{}

	if len(l.OtherLease("member-1")) == 0 {
		t.Error("other lease  empty")
	}

	if len(l.StatusMemberPrefix()) == 0 {
		t.Error("StatusMemberPrefix  empty")
	}

	if len(l.OtherStatusMemberKey("member-1")) == 0 {
		t.Error("other status member key empty")
	}

	if len(l.StatusObjectsPrefix()) == 0 {
		t.Error("StatusObjectsPrefix empty")
	}

	if len(l.StatusObjectKey("default", "member-1")) == 0 {
		t.Error("StatusObjectKey key empty")
	}

	if len(l.ConfigObjectPrefix()) == 0 {
		t.Error("ConfigObjectPrefix empty")
	}

	if len(l.ConfigObjectKey("member-1")) == 0 {
		t.Error("ConfigObjectKey empty")
	}

	if len(l.ConfigVersion()) == 0 {
		t.Error("ConfigVersion empty")
	}

	if len(l.WasmCodeEvent()) == 0 {
		t.Error("WasmCodeEvent empty")
	}

	if len(l.WasmDataPrefix("pipeline", "wasm")) == 0 {
		t.Error("WasmDataPrefix empty")
	}

	assert.Equal(customDataPrefix, l.CustomDataPrefix())
	assert.Equal(customDataKindPrefix, l.CustomDataKindPrefix())
}
