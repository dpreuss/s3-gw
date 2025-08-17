// Copyright (c) 2025 Starfish Storage, Inc.
//
// This file is part of the VersityGW project developed by Starfish Storage, Inc.
// This file was assisted by Gemini AI.
//
// The VersityGW project is licensed under the Apache License, version 2.0
// (the "License"); you may not use this file except in compliance with the
// License. You may obtain a copy of the License at:
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package starfish

import "fmt"

// ErrStarfishAPIAccess indicates a failure to access the Starfish API
type ErrStarfishAPIAccess struct {
	StatusCode int
	Msg        string
}

func (e *ErrStarfishAPIAccess) Error() string {
	return fmt.Sprintf("starfish API access error: status %d, msg: %s", e.StatusCode, e.Msg)
}

// ErrInvalidQuery indicates a malformed or invalid query
type ErrInvalidQuery struct {
	Reason string
}

func (e *ErrInvalidQuery) Error() string {
	return fmt.Sprintf("invalid starfish query: %s", e.Reason)
}
