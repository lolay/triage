// Package engine defines the check execution model and runner.
//
// m1 ships the interfaces (Check, Result, and the Severity ladder
// error/warn/info) plus a runner that executes the active profile's checks in
// order — empty in m1, since no real check types exist yet. The first real
// check type (tool) and severity/group rendering land in m2.
package engine
