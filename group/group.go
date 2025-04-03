// Package group provides a way to group methods and register them all at once.
//
// It simplifies error handling when registering multiple methods, and allows to use nested groups.
package group

import "github.com/kytnacode/go-jrpc"

// DefaultSeparator is the default separator for the subgroups, see [Group.SetSeparator] to use a custom separator.
const DefaultSeparator = "."

// Group is a group of methods. It allows to add methods with [Group.Register] or [Group.AddMethod] and defer error handling
// until all methods are registered to another register with [Group.RegisterTo]. It also allows to use nested groups with [Group.Use],
// nested groups will have a prefix for the methods separated by a separator, defaults to [DefaultSeparator], can be changed with
// [Group.SetSeparator].
//
// Implements the [jrpc.Register] interface, but if you are using group directly, prefer to use [Group.AddMethod] instead of [Group.Register],
// as Register will never return an error, errors will be returned all together by [Group.RegisterTo], [Group.AddMethod] is equivalent to [Group.Register].
//
// The zero value is ready to use.
//
// Example:
//
//	var g group.Group
//	g.AddMethod("method1", handler1)
//	g.AddMethod("method2", handler2)
//
//	g.Use("prefix", func(subG *group.Group) {
//	    subG.AddMethod("method3", handler3)
//	    subG.AddMethod("method4", handler4)
//	})
//
//	errs := g.RegisterTo(server)
//	if len(errs) > 0 {
//	    // Handle errors
//	}
type Group struct {
	handlers map[string]any
	sep      string // Subgroup separator
}

// init initializes the group, check if the group is already initialized is responsibility of the caller, if g.handlers is nil, the group is not initialized yet.
func (g *Group) init() {
	g.handlers = make(map[string]any)
	g.sep = DefaultSeparator
}

// SetSeparator sets the separator from the group from the parent group, and subgroups when using [Group.Use]. Defaults to [DefaultSeparator].
func (g *Group) SetSeparator(sep string) {
	if g.handlers == nil { // if g.handlers is nil, the group is not initialized
		g.init()
	}

	g.sep = sep
}

// Register registers a method to the group. Implements the [jrpc.Register] interface.
//
// error always returns nil, errors will be returned all together by [Group.RegisterTo], prefer to use [Group.AddMethod] that don't return nothing
// to avoid confusion:
//
//	// _ = g.Register("method", handler)
//	g.AddMethod("method", handler) // Prefer this
func (g *Group) Register(method string, handler any) error {
	if g.handlers == nil { // if g.handlers is nil, the group is not initialized
		g.init()
	}

	g.handlers[method] = handler // Defer registering.

	return nil
}

// AddMethod adds a method to the group. Prefer to use this instead of [Group.Register] to avoid confusion.
func (g *Group) AddMethod(method string, handler any) {
	if g.handlers == nil { // if g.handlers is nil, the group is not initialized
		g.init()
	}

	g.handlers[method] = handler // Defer registering.
}

// Use add a subgroup of methods to the group. The methods of the subgroup will have a prefix separated by the separator, defaults to [DefaultSeparator],
// if subgroup calls [Group.SetSeparator] the separator will be changed for the subgroup and all subgroups of the subgroup, if prefix is empty, subgroup
// methods will be added without prefix nor separator, whether the separator is changed or not:
//
//	 g.Use("math", func(subG *group.Group) {
//	     subG.AddMethod("add", addHandler) // Will be registered as "math.add"
//	     subG.AddMethod("sub", subHandler) // Will be registered as "math.sub"
//	 })
//
//		g.Use("strings", func(subG *group.Group) {
//	   subG.SetSeparator("/") // Change separator for the subgroup
//	   subG.AddMethod("concat", concatHandler) // Will be registered as "strings/concat"
//	   subG.AddMethod("split", splitHandler) // Will be registered as "strings/split"
//	 })
//
//	 g.Use("", func(subG *group.Group) {
//	     subG.AddMethod("method1", handler1) // Will be registered as "method1"
//	     subG.AddMethod("method2", handler2) // Will be registered as "method2"
//	 })
func (g *Group) Use(prefix string, useG func(subG *Group)) {
	if g.handlers == nil { // if g.handlers is nil, the group is not initialized
		g.init()
	}

	subG := new(Group)
	subG.SetSeparator(g.sep) // Subgroup separators default to parent's separator

	useG(subG)

	var pre string
	if prefix != "" {
		pre = prefix + subG.sep
	} else {
		pre = "" // Ignore separator if prefix is empty
	}

	// Add the methods of the subgroup to the parent group.
	for method, handler := range subG.handlers {
		g.handlers[pre+method] = handler
	}
}

// RegisterTo registers all methods of the group to a [jrpc.Register], returns a slice with all errors ocurred during the registration,
// is equivalent to call [jrpc.Register.Register] for each method of the group, and append the errors to a slice, and return the slice,
// to add methods to the group use [Group.AddMethod] or [Group.Register]:
//
//	var g group.Group
//
//	g.AddMethod("method1", handler1)
//	g.AddMethod("method2", handler2)
//
//	errs := g.RegisterTo(server)
//	if len(errs) > 0 {
//	    // Handle errors
//	}
func (g *Group) RegisterTo(r jrpc.Register) []error {
	errs := make([]error, 0, len(g.handlers))

	for method, handler := range g.handlers {
		if err := r.Register(method, handler); err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}
