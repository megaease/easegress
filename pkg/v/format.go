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

package v

import (
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"time"
)

var (
	formatsFuncs = map[string]FormatFunc{
		"urlname":          urlName,
		"httpmethod":       httpMethod,
		"httpmethod-array": httpMethodArray,
		"httpcode":         httpCode,
		"httpcode-array":   httpCodeArray,
		"timerfc3339":      timerfc3339,
		"duration":         duration,
		"ipcidr":           ipcidr,
		"ipcidr-array":     ipcidrArray,
		"hostport":         hostport,
		"regexp":           _regexp,
		"base64":           _base64,
		"url":              _url,
	}

	urlCharsRegexp = regexp.MustCompile(`^[A-Za-z0-9\-_\.~]{1,253}$`)
)

func getFormatFunc(format string) (FormatFunc, bool) {
	switch format {
	case "date-time", "email", "hostname", "ipv4", "ipv6", "uri":
		return standardFormat, true

	case "":
		// NOTICE: Empty format does nothing like standard format.
		return standardFormat, true
	}

	if fn, exists := formatsFuncs[format]; exists {
		return fn, true
	}

	return nil, false
}

func standardFormat(v interface{}) error {
	// NOTICE: Its errors will be reported by standard json schema.
	return nil
}

func urlName(v interface{}) error {
	if urlCharsRegexp.MatchString(v.(string)) {
		return nil
	}

	return fmt.Errorf("invalid name format: %s", v)
}

func httpMethod(v interface{}) error {
	switch v.(string) {
	case http.MethodGet,
		http.MethodHead,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
		http.MethodConnect,
		http.MethodOptions,
		http.MethodTrace:
		return nil
	default:
		return fmt.Errorf("invalid http method")
	}
}

func httpMethodArray(v interface{}) error {
	for _, method := range v.([]string) {
		err := httpMethod(method)
		if err != nil {
			return err
		}
	}

	return nil
}

func httpCode(v interface{}) error {
	code := v.(int)
	// Reference: https://tools.ietf.org/html/rfc7231#section-6
	if code < 100 || code >= 600 {
		return fmt.Errorf("invalid http code")
	}
	return nil
}

func httpCodeArray(v interface{}) error {
	for _, method := range v.([]int) {
		err := httpCode(method)
		if err != nil {
			return err
		}
	}

	return nil
}

func timerfc3339(v interface{}) error {
	s := v.(string)
	_, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return fmt.Errorf("invalid RFC3339 time: %v", err)
	}
	return nil
}

func duration(v interface{}) error {
	s := v.(string)
	_, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration: %v", err)
	}
	return nil
}

func ipcidr(v interface{}) error {
	s := v.(string)
	ip := net.ParseIP(s)
	if ip != nil {
		return nil
	}

	_, _, err := net.ParseCIDR(s)
	if err != nil {
		return fmt.Errorf("invalid ip or cidr")
	}

	return nil
}

func ipcidrArray(v interface{}) error {
	for _, ic := range v.([]string) {
		err := ipcidr(ic)
		if err != nil {
			return err
		}
	}

	return nil
}

func hostport(v interface{}) error {
	s := v.(string)
	_, _, err := net.SplitHostPort(s)
	if err != nil {
		return fmt.Errorf("invalid hostport: %v", err)
	}
	return nil
}

func _regexp(v interface{}) error {
	s := v.(string)
	_, err := regexp.Compile(s)
	if err != nil {
		return fmt.Errorf("invalid regular expression: %v", err)
	}

	return nil
}

func _base64(v interface{}) error {
	s := v.(string)
	_, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return fmt.Errorf("invalid base64: %v", err)
	}

	return nil
}

func _url(v interface{}) error {
	s := v.(string)
	_, err := url.Parse(s)
	if err != nil {
		return fmt.Errorf("invalid url: %v", err)
	}

	return nil
}
