package engine

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/lolay/triage/internal/config"
)

var tmplRe = regexp.MustCompile(`\{\{\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*\}\}`)

// expand replaces every {{ name }} token in s using vars. It is fail-closed:
// a malformed token (any "{{" that does not begin a well-formed {{ name }}) and
// an unknown variable both return an error rather than passing through silently.
func expand(s string, vars map[string]string) (string, error) {
	if s == "" || !strings.Contains(s, "{{") {
		return s, nil
	}
	if frag, ok := malformedToken(s); ok {
		return "", fmt.Errorf("malformed template %q - expected {{ name }} where name is a "+
			"letter or underscore followed by letters, digits, or underscores", frag)
	}
	var err error
	out := tmplRe.ReplaceAllStringFunc(s, func(token string) string {
		if err != nil {
			return token
		}
		m := tmplRe.FindStringSubmatch(token)
		if m == nil {
			return token
		}
		name := m[1]
		val, ok := vars[name]
		if !ok {
			err = fmt.Errorf("unknown variable {{ %s }} - define it under vars: or pass --var %s=value", name, name)
			return token
		}
		return val
	})
	return out, err
}

// malformedToken reports whether s contains a "{{" that does not begin a
// well-formed {{ name }} token, returning a short fragment for the error. This
// keeps expansion fail-closed: typo'd or unsupported template shapes (e.g.
// "{{ 1bad }}", "{{ a-b }}", "{{}}", an unclosed "{{") fail the check instead
// of silently surviving as literal text.
func malformedToken(s string) (string, bool) {
	valid := make(map[int]bool)
	for _, m := range tmplRe.FindAllStringIndex(s, -1) {
		valid[m[0]] = true
	}
	for i := 0; i+1 < len(s); i++ {
		if s[i] == '{' && s[i+1] == '{' && !valid[i] {
			return tokenFragment(s, i), true
		}
	}
	return "", false
}

// tokenFragment returns a short, readable slice of s starting at the "{{" at
// index i, ending just past the next "}}" (or bounded if unclosed).
func tokenFragment(s string, i int) string {
	rest := s[i:]
	if end := strings.Index(rest, "}}"); end >= 0 {
		return rest[:end+2]
	}
	const maxLen = 20
	if len(rest) > maxLen {
		return rest[:maxLen]
	}
	return rest
}

// expandCheck returns a copy of c with all string fields expanded via vars.
// Items and OneOf are left for recursive expansion in runCheck.
func expandCheck(c config.Check, vars map[string]string) (config.Check, error) {
	var err error
	expandField := func(s string) string {
		if err != nil || s == "" {
			return s
		}
		out, e := expand(s, vars)
		if e != nil {
			err = e
			return s
		}
		return out
	}

	out := c
	out.Value = expandField(c.Value)
	out.Version = expandField(c.Version)
	out.Constraint = expandField(c.Constraint)
	out.Group = expandField(c.Group)
	out.Hint = expandField(c.Hint)
	out.Dir = expandField(c.Dir)
	out.Label = expandField(c.Label)
	out.Matches = expandField(c.Matches)
	out.Contains = expandField(c.Contains)
	out.Config = expandField(c.Config)

	if len(c.WithEnv) > 0 {
		out.WithEnv = make(map[string]string, len(c.WithEnv))
		for k, v := range c.WithEnv {
			out.WithEnv[k] = expandField(v)
		}
	}

	if err != nil {
		return c, err
	}
	return out, nil
}
