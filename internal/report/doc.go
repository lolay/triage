// Package report renders check results for humans and machines.
//
// m1 ships the skeletons: a human board (header + summary line) and a --json
// reporter (struct + encoder). Group nesting, in-flight pending lines, and the
// delegate tree (spec §7.2) are deferred to m2.
package report
