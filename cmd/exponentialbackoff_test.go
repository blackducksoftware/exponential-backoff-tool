/*
* Copyright 2020-present, Synopsys, Inc. * All rights reserved.
*
* This source code is licensed under the Apache-2.0 license found in
* the LICENSE file in the root directory of this source tree. */

package cmd

import (
	"regexp"
	"testing"
)

func TestExponentialBackoff(*testing.T) {
	var retryCodes []int
	var retryStrings []string
	var retryRegexps []*regexp.Regexp
	var command []string
	command = append(command, "echo")
	command = append(command, "hi")

	ExponentialBackoff(command, "1", 4, 10, true, retryCodes, retryStrings, retryRegexps, "")
}
