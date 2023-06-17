package cli

import (
	"context"
)

// Context can be used to retrieve context-specific args and parsed command-line options.
type Context struct {
	context.Context
	*App
	Command *Command
	args    *Args
	parent  *Context
}

func newContext(parentCtx context.Context, app *App) *Context {
	return &Context{
		Context: parentCtx,
		App:     app,
	}
}

func (ctx *Context) Clone(command *Command, args []string) *Context {
	return &Context{
		Context: ctx.Context,
		App:     ctx.App,
		Command: command,
		args:    newArgs(args),
		parent:  ctx,
	}
}

func (ctx Context) WithValue(key, val any) *Context {
	ctx.Context = context.WithValue(ctx.Context, key, val)
	return &ctx
}

func (ctx *Context) Value(key any) any {
	return ctx.Context.Value(key)
}

// Args returns the command line arguments associated with the context.
func (ctx *Context) Args() *Args {
	return ctx.args
}

func (ctx *Context) Parent() *Context {
	return ctx.parent
}
