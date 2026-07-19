// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2026 hchw

package bot

type Translator interface {
	ParseIncoming(raw []byte) (*Message, error)
	Platform() string
}
