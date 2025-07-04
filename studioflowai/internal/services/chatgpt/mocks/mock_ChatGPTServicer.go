// Code generated by mockery; DO NOT EDIT.
// github.com/vektra/mockery
// template: testify

package services

import (
	"context"

	"github.com/gnzdotmx/studioflowai/studioflowai/internal/services/chatgpt"
	mock "github.com/stretchr/testify/mock"
)

// NewMockChatGPTServicer creates a new instance of MockChatGPTServicer. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockChatGPTServicer(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockChatGPTServicer {
	mock := &MockChatGPTServicer{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

// MockChatGPTServicer is an autogenerated mock type for the ChatGPTServicer type
type MockChatGPTServicer struct {
	mock.Mock
}

type MockChatGPTServicer_Expecter struct {
	mock *mock.Mock
}

func (_m *MockChatGPTServicer) EXPECT() *MockChatGPTServicer_Expecter {
	return &MockChatGPTServicer_Expecter{mock: &_m.Mock}
}

// Complete provides a mock function for the type MockChatGPTServicer
func (_mock *MockChatGPTServicer) Complete(ctx context.Context, messages []services.ChatMessage, opts services.CompletionOptions) (*services.ChatResponse, error) {
	ret := _mock.Called(ctx, messages, opts)

	if len(ret) == 0 {
		panic("no return value specified for Complete")
	}

	var r0 *services.ChatResponse
	var r1 error
	if returnFunc, ok := ret.Get(0).(func(context.Context, []services.ChatMessage, services.CompletionOptions) (*services.ChatResponse, error)); ok {
		return returnFunc(ctx, messages, opts)
	}
	if returnFunc, ok := ret.Get(0).(func(context.Context, []services.ChatMessage, services.CompletionOptions) *services.ChatResponse); ok {
		r0 = returnFunc(ctx, messages, opts)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*services.ChatResponse)
		}
	}
	if returnFunc, ok := ret.Get(1).(func(context.Context, []services.ChatMessage, services.CompletionOptions) error); ok {
		r1 = returnFunc(ctx, messages, opts)
	} else {
		r1 = ret.Error(1)
	}
	return r0, r1
}

// MockChatGPTServicer_Complete_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Complete'
type MockChatGPTServicer_Complete_Call struct {
	*mock.Call
}

// Complete is a helper method to define mock.On call
//   - ctx context.Context
//   - messages []services.ChatMessage
//   - opts services.CompletionOptions
func (_e *MockChatGPTServicer_Expecter) Complete(ctx interface{}, messages interface{}, opts interface{}) *MockChatGPTServicer_Complete_Call {
	return &MockChatGPTServicer_Complete_Call{Call: _e.mock.On("Complete", ctx, messages, opts)}
}

func (_c *MockChatGPTServicer_Complete_Call) Run(run func(ctx context.Context, messages []services.ChatMessage, opts services.CompletionOptions)) *MockChatGPTServicer_Complete_Call {
	_c.Call.Run(func(args mock.Arguments) {
		var arg0 context.Context
		if args[0] != nil {
			arg0 = args[0].(context.Context)
		}
		var arg1 []services.ChatMessage
		if args[1] != nil {
			arg1 = args[1].([]services.ChatMessage)
		}
		var arg2 services.CompletionOptions
		if args[2] != nil {
			arg2 = args[2].(services.CompletionOptions)
		}
		run(
			arg0,
			arg1,
			arg2,
		)
	})
	return _c
}

func (_c *MockChatGPTServicer_Complete_Call) Return(chatResponse *services.ChatResponse, err error) *MockChatGPTServicer_Complete_Call {
	_c.Call.Return(chatResponse, err)
	return _c
}

func (_c *MockChatGPTServicer_Complete_Call) RunAndReturn(run func(ctx context.Context, messages []services.ChatMessage, opts services.CompletionOptions) (*services.ChatResponse, error)) *MockChatGPTServicer_Complete_Call {
	_c.Call.Return(run)
	return _c
}

// GetContent provides a mock function for the type MockChatGPTServicer
func (_mock *MockChatGPTServicer) GetContent(ctx context.Context, messages []services.ChatMessage, opts services.CompletionOptions) (string, error) {
	ret := _mock.Called(ctx, messages, opts)

	if len(ret) == 0 {
		panic("no return value specified for GetContent")
	}

	var r0 string
	var r1 error
	if returnFunc, ok := ret.Get(0).(func(context.Context, []services.ChatMessage, services.CompletionOptions) (string, error)); ok {
		return returnFunc(ctx, messages, opts)
	}
	if returnFunc, ok := ret.Get(0).(func(context.Context, []services.ChatMessage, services.CompletionOptions) string); ok {
		r0 = returnFunc(ctx, messages, opts)
	} else {
		r0 = ret.Get(0).(string)
	}
	if returnFunc, ok := ret.Get(1).(func(context.Context, []services.ChatMessage, services.CompletionOptions) error); ok {
		r1 = returnFunc(ctx, messages, opts)
	} else {
		r1 = ret.Error(1)
	}
	return r0, r1
}

// MockChatGPTServicer_GetContent_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetContent'
type MockChatGPTServicer_GetContent_Call struct {
	*mock.Call
}

// GetContent is a helper method to define mock.On call
//   - ctx context.Context
//   - messages []services.ChatMessage
//   - opts services.CompletionOptions
func (_e *MockChatGPTServicer_Expecter) GetContent(ctx interface{}, messages interface{}, opts interface{}) *MockChatGPTServicer_GetContent_Call {
	return &MockChatGPTServicer_GetContent_Call{Call: _e.mock.On("GetContent", ctx, messages, opts)}
}

func (_c *MockChatGPTServicer_GetContent_Call) Run(run func(ctx context.Context, messages []services.ChatMessage, opts services.CompletionOptions)) *MockChatGPTServicer_GetContent_Call {
	_c.Call.Run(func(args mock.Arguments) {
		var arg0 context.Context
		if args[0] != nil {
			arg0 = args[0].(context.Context)
		}
		var arg1 []services.ChatMessage
		if args[1] != nil {
			arg1 = args[1].([]services.ChatMessage)
		}
		var arg2 services.CompletionOptions
		if args[2] != nil {
			arg2 = args[2].(services.CompletionOptions)
		}
		run(
			arg0,
			arg1,
			arg2,
		)
	})
	return _c
}

func (_c *MockChatGPTServicer_GetContent_Call) Return(s string, err error) *MockChatGPTServicer_GetContent_Call {
	_c.Call.Return(s, err)
	return _c
}

func (_c *MockChatGPTServicer_GetContent_Call) RunAndReturn(run func(ctx context.Context, messages []services.ChatMessage, opts services.CompletionOptions) (string, error)) *MockChatGPTServicer_GetContent_Call {
	_c.Call.Return(run)
	return _c
}
