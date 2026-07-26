// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 hchw

package bot

type Translator interface {
	ParseIncoming(raw []byte) (*Message, error)
	Platform() string
}
