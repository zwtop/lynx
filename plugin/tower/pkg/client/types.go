/*
Copyright 2021 The Lynx Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package client

import (
	"encoding/json"
	"fmt"
)

type Request struct {
	Query         string                 `json:"query"`
	OperationName string                 `json:"operationName,omitempty"`
	Variables     map[string]interface{} `json:"variables,omitempty"`
}

type Response struct {
	Data   json.RawMessage `json:"data"`
	Errors []ResponseError `json:"errors,omitempty"`
}

type ResponseError struct {
	Message string    `json:"message"`
	Code    ErrorCode `json:"code,omitempty"`
}

type ErrorCode string

const (
	LoginFailed           ErrorCode = "LOGIN_FAILED"
	UserNotFound          ErrorCode = "USER_NOT_FOUND"
	UserPasswordIncorrect ErrorCode = "USER_PASSWORD_INCORRECT"
	NotMatchUser          ErrorCode = "NOT_MATCH_USER"
	LoadTokenFailed       ErrorCode = "LOAD_TOKEN_FAILED"
	WebsocketConnectError ErrorCode = "WEBSOCKET_CONNECT_ERROR"
)

func (e ResponseError) Error() string {
	return fmt.Sprintf("message: %s, errcode: %s", e.Message, e.Code)
}

func HasAuthError(errors []ResponseError) bool {
	for _, err := range errors {
		switch err.Code {
		case LoginFailed, UserNotFound, UserPasswordIncorrect, NotMatchUser, LoadTokenFailed:
			return true
		}
	}
	return false
}

// Message is the request/reponse type when use the websocket connection
type Message struct {
	ID   string      `json:"id"`
	Type MessageType `json:"type"`

	PayLoad json.RawMessage `json:"payload"`
}

type MessageType string

const (
	StartMsg    MessageType = "start"
	DataMsg     MessageType = "data"
	ErrorMsg    MessageType = "error"
	CompleteMsg MessageType = "complete"
)

type AuthInformation struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Source   string `json:"source"`
}

// MutationEvent is the event subscribed from tower
type MutationEvent struct {
	Mutation MutationType    `json:"mutation"`
	Node     json.RawMessage `json:"node"`
}

type MutationType string

const (
	CreateEvent MutationType = "CREATED"
	DeleteEvent MutationType = "DELETED"
	UpdateEvent MutationType = "UPDATED"
)
