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
	"fmt"
	"strconv"
	"strings"
	"text/template"

	"github.com/megaease/easegress/pkg/logger"
)

func toFloat64(val interface{}) float64 {
	switch v := val.(type) {
	case int8:
		return float64(v)
	case int16:
		return float64(v)
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	case int:
		return float64(v)
	case uint8:
		return float64(v)
	case uint16:
		return float64(v)
	case uint32:
		return float64(v)
	case uint64:
		return float64(v)
	case uint:
		return float64(v)
	case uintptr:
		return float64(v)
	case json.Number:
		f, e := v.Float64()
		if e == nil {
			return f
		}
		panic(e)
	case string:
		f, e := strconv.ParseFloat(v, 64)
		if e == nil {
			return f
		}
		panic(e)
	}
	panic(fmt.Errorf("cannot convert %v to float64", val))
}

func mergeObject(objs ...map[string]interface{}) interface{} {
	out := map[string]interface{}{}

	var merge func(a, b map[string]interface{})
	merge = func(a, b map[string]interface{}) {
		for k, v := range b {
			ma, oka := a[k].(map[string]interface{})
			mb, okb := v.(map[string]interface{})
			if oka && okb {
				merge(ma, mb)
			} else {
				a[k] = v
			}
		}
	}

	for _, obj := range objs {
		merge(out, obj)
	}

	return out
}

var extraFuncs = template.FuncMap{
	"addf": func(a, b interface{}) float64 {
		x, y := toFloat64(a), toFloat64(b)
		return x + y
	},

	"subf": func(a, b interface{}) float64 {
		x, y := toFloat64(a), toFloat64(b)
		return x - y
	},

	"mulf": func(a, b interface{}) float64 {
		x, y := toFloat64(a), toFloat64(b)
		return x * y
	},

	"divf": func(a, b interface{}) float64 {
		x, y := toFloat64(a), toFloat64(b)
		if y == 0 {
			panic("divisor is zero")
		}
		return x / y
	},

	"log": func(level, msg string) string {
		switch strings.ToLower(level) {
		case "debug":
			logger.Debugf(msg)
		case "info":
			logger.Infof(msg)
		case "warn":
			logger.Warnf(msg)
		case "error":
			logger.Errorf(msg)
		}
		return ""
	},

	"mergeObject": mergeObject,

	"jsonEscape": func(s string) string {
		b, err := json.Marshal(s)
		if err != nil {
			panic(err)
		}
		return string(b[1 : len(b)-1])
	},
}
