package validate

import (
	"context"
	"errors"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
)

// NewInterceptor returns a Connect interceptor that validates request messages
// using protovalidate constraints defined in the proto files.
func NewInterceptor() connect.Interceptor {
	return &interceptor{}
}

type interceptor struct{}

func (i *interceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		if msg, ok := req.Any().(proto.Message); ok {
			if err := protovalidate.Validate(msg); err != nil {
				var ve *protovalidate.ValidationError
				if errors.As(err, &ve) {
					return nil, connect.NewError(connect.CodeInvalidArgument, ve)
				}
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		}
		return next(ctx, req)
	}
}

func (i *interceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *interceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		// For server-streaming RPCs, the initial request is already sent.
		// Validation of streaming messages would need per-message interception
		// which isn't needed for our current RPCs (SendMessage is the only stream
		// and it's server-streaming with a single request message).
		return next(ctx, conn)
	}
}
