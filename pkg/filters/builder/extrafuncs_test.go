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

package builder

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToFloat64(t *testing.T) {
	assert := assert.New(t)
	assert.Equal(float64(1), toFloat64(int8(1)))
	assert.Equal(float64(1), toFloat64(int16(1)))
	assert.Equal(float64(1), toFloat64(int32(1)))
	assert.Equal(float64(1), toFloat64(int64(1)))
	assert.Equal(float64(1), toFloat64(int(1)))
	assert.Equal(float64(1), toFloat64(uint8(1)))
	assert.Equal(float64(1), toFloat64(uint16(1)))
	assert.Equal(float64(1), toFloat64(uint32(1)))
	assert.Equal(float64(1), toFloat64(uint64(1)))
	assert.Equal(float64(1), toFloat64(uint(1)))
	assert.Equal(float64(1), toFloat64(uintptr(1)))
	assert.Equal(float64(123), toFloat64(json.Number("123")))
	assert.Equal(float64(1), toFloat64("1"))
	assert.Panics(func() { toFloat64("s") })
	assert.Panics(func() { toFloat64(func() {}) })
	assert.Panics(func() { toFloat64(json.Number("not a number")) })
}

func TestExtraFuncs(t *testing.T) {
	assert := assert.New(t)
	assert.Equal(float64(123), extraFuncs["addf"].(func(a, b interface{}) float64)(120, 3))
	assert.Equal(float64(117), extraFuncs["subf"].(func(a, b interface{}) float64)(120, 3))
	assert.Equal(float64(360), extraFuncs["mulf"].(func(a, b interface{}) float64)(120, 3))
	assert.Equal(float64(40), extraFuncs["divf"].(func(a, b interface{}) float64)(120, 3))
	assert.Panics(func() { extraFuncs["divf"].(func(a, b interface{}) float64)(120, 0) })

	assert.Equal("", extraFuncs["log"].(func(level, msg string) string)("debug", "debug"))
	assert.Equal("", extraFuncs["log"].(func(level, msg string) string)("info", "info"))
	assert.Equal("", extraFuncs["log"].(func(level, msg string) string)("warn", "warn"))
	assert.Equal("", extraFuncs["log"].(func(level, msg string) string)("error", "error"))

	assert.Equal(`abcd\"ABCD`, extraFuncs["jsonEscape"].(func(s string) string)(`abcd"ABCD`))
}

func TestMergeObject(t *testing.T) {
	assert := assert.New(t)

	obj1 := map[string]interface{}{
		"a": 1,
		"b": 2,
		"e": map[string]interface{}{
			"e1": 10,
			"e3": 5,
		},
		"f": map[string]interface{}{
			"f1": 10,
		},
	}

	obj2 := map[string]interface{}{
		"b": 3,
		"c": 4,
		"d": 1.5,
		"e": map[string]interface{}{
			"e1": 3,
			"e2": 4,
		},
		"f": "abcd",
	}

	result := mergeObject(obj1, obj2, nil)
	m := result.(map[string]interface{})

	assert.Equal(1, m["a"])
	assert.Equal(3, m["b"])
	assert.Equal(4, m["c"])
	assert.Equal(1.5, m["d"])
	assert.Equal("abcd", m["f"])

	m = m["e"].(map[string]interface{})
	assert.Equal(3, m["e1"])
	assert.Equal(4, m["e2"])
	assert.Equal(5, m["e3"])
}
