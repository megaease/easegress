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

package context

import "net/http"

// the error code list which align with the HTTP status code
const (
	EGStatusContinue              = http.StatusContinue
	EGStatusSwitchingProtocols    = http.StatusSwitchingProtocols
	EGStatusProcessing            = http.StatusProcessing
	EGStatusOK                    = http.StatusOK
	EGStatusCreated               = http.StatusCreated
	EGStatusAccepted              = http.StatusAccepted
	EGStatusNoContent             = http.StatusNoContent
	EGStatusPartialContent        = http.StatusPartialContent
	EGStatusSpecialResponse       = 300 // http.StatusMultipleChoices
	EGStatusMovedPermanently      = http.StatusMovedPermanently
	EGStatusMovedTemporarily      = 302 // http.StatusFound
	EGStatusSeeOther              = http.StatusSeeOther
	EGStatusNotModified           = http.StatusNotModified
	EGStatusTemporaryRedirect     = http.StatusTemporaryRedirect
	EGStatusBadRequest            = http.StatusBadRequest
	EGStatusUnauthorized          = http.StatusUnauthorized
	EGStatusForbidden             = http.StatusForbidden
	EGStatusNotFound              = http.StatusNotFound
	EGStatusNotAllowed            = http.StatusMethodNotAllowed
	EGStatusRequestTimeOut        = http.StatusRequestTimeout
	EGStatusConflict              = http.StatusConflict
	EGStatusLengthRequired        = http.StatusLengthRequired
	EGStatusPreconditionFailed    = http.StatusPreconditionFailed
	EGStatusRequestEntityTooLarge = http.StatusRequestEntityTooLarge
	EGStatusRequestURITooLong     = http.StatusRequestURITooLong
	EGStatusUnsupportedMediaType  = http.StatusUnsupportedMediaType
	EGStatusRangeNotSatisfiable   = http.StatusRequestedRangeNotSatisfiable
	EGStatusTooManyRequests       = http.StatusTooManyRequests
	EGStatusClose                 = 444
	EGStatusEgCodes               = 494
	EGStatusRequestHeaderTooLarge = 494
	EGStatusHTTPSCertError        = 495
	EGStatusHTTPSNoCert           = 496
	EGStatusToHTTPS               = 497
	EGStatusClientClosedRequest   = 499
	EGStatusInternalServerError   = http.StatusInternalServerError
	EGStatusNotImplemented        = http.StatusNotImplemented
	EGStatusBadGateway            = http.StatusBadGateway
	EGStatusServiceUnavailable    = http.StatusServiceUnavailable
	EGStatusGatewayTimeOut        = http.StatusGatewayTimeout
	EGStatusInsufficientStorage   = http.StatusInsufficientStorage
	EGStatusBadResponse           = 599
)

// categories of status code
const (
	egNetworkError = 1 << iota
)

// map status code to categories, note one status code could belong to
// multiple categories
var (
	statusCodeCategory = map[int]uint8{
		EGStatusClientClosedRequest: egNetworkError,
		EGStatusServiceUnavailable:  egNetworkError,
		EGStatusBadGateway:          egNetworkError,
		EGStatusGatewayTimeOut:      egNetworkError,
		EGStatusRequestTimeOut:      egNetworkError,
	}
)

// IsNetworkError returns if the error is network type.
func IsNetworkError(code int) bool {
	c, ok := statusCodeCategory[code]
	return ok && (c&egNetworkError) != 0
}
