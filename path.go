package goxmlpath

//package main
import (
	"fmt"
	"strconv"
)

type Path struct {
	path  string
	steps []pathStep
}
type pathCompiler struct {
	path string
	//i指path中字符序列号 path[i]
	i int
}
type pathStep struct {
	//是否是根节点
	root bool
	axis string
	name string
	kind int
	pred predicate
}
// predicate is a marker interface for predicate types.
type predicate interface {
	predicate()
}
//pathCompiler methods
func (c *pathCompiler) errorf(format string, args ...interface{}) error {
	return fmt.Errorf("compiling xml path %q:%d: %s", c.path, c.i, fmt.Sprintf(format, args...))
}

//判断pathCompiler中第i个的值知否为空
func (c *pathCompiler) skipSpaces() bool {
	mark := c.i
	for c.i < len(c.path) {
		if c.path[c.i] != ' ' {
			break
		}
		c.i++
	}
	return c.i != mark
}

func Compile(path string) (*Path, error) {
	c := pathCompiler{path, 0}
	if path == "" {
		return nil, c.errorf("empty path")
	}
	p, err := c.parsePath()
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (c *pathCompiler) skipByte(b byte) bool {
	if c.i < len(c.path) && c.path[c.i] == b {
		c.i++
		return true
	}
	return false
}
func (c *pathCompiler) peekByte(b byte) bool {
	return c.i < len(c.path) && c.path[c.i] == b
}
func (c *pathCompiler) parsePath() (path *Path, err error) {
	var steps []pathStep
	var start = c.i
	for {
		step := pathStep{axis: "child"}

		c.skipSpaces()
		if c.i == 0 && c.skipByte('/') {
			c.skipSpaces()
			step.root = true
			if len(c.path) == 1 {
				step.name = "*"
			}
		}
		if c.peekByte('/') {
			step.axis = "descendant-or-self"
			step.name = "*"
		} else if c.skipByte('@') {
			mark := c.i
			if !c.skipName() {
				return nil, c.errorf("missing name after @")
			}
			step.axis = "attribute"
			step.name = c.path[mark:c.i]
			step.kind = attrNode
		} else {
			mark := c.i
			if c.skipName() {
				step.name = c.path[mark:c.i]
				c.skipSpaces()
			}
			if step.name == "" {
				return nil, c.errorf("missing name")
			} else if step.name == "*" {
				step.kind = startNode
			} else if step.name == "." {
				step.axis = "self"
				step.name = "*"
			} else if step.name == ".." {
				step.axis = "parent"
				step.name = "*"
			} else {
				if c.skipByte(':') {
					if !c.skipByte(':') {
						return nil, c.errorf("missing ':'")
					}
					c.skipSpaces()
					switch step.name {
					case "attribute":
						step.kind = attrNode
					case "self", "child", "parent":
					case "descendant", "descendant-or-self":
					case "ancestor", "ancestor-or-self":
					case "following", "following-sibling":
					case "preceding", "preceding-sibling":
					default:
						return nil, c.errorf("unsupported axis: %q", step.name)
					}
					step.axis = step.name

					mark = c.i
					if !c.skipName() {
						return nil, c.errorf("missing name")
					}
					step.name = c.path[mark:c.i]

					c.skipSpaces()
				}
				if c.skipByte('(') {
					c.skipSpaces()
					conflict := step.kind != anyNode
					switch step.name {
					case "node":
					// must be anyNode
					case "text":
						step.kind = textNode
					case "comment":
						step.kind = commentNode
					case "processing-instruction":
						step.kind = procInstNode
					default:
						return nil, c.errorf("unsupported expression: %s()", step.name)
					}
					if conflict {
						return nil, c.errorf("%s() cannot succeed on axis %q", step.name, step.axis)
					}

					name := step.name
					literal, err := c.parseLiteral()
					if err == errNoLiteral {
						step.name = "*"
					} else if err != nil {
						return nil, c.errorf("%v", err)
					} else if step.kind == procInstNode {
						c.skipSpaces()
						step.name = literal
					} else {
						return nil, c.errorf("%s() has no arguments", name)
					}
					if !c.skipByte(')') {
						return nil, c.errorf("%s() missing ')'", name)
					}
					c.skipSpaces()
				} else if step.name == "*" && step.kind == anyNode {
					step.kind = startNode
				}
			}
		}
		if c.skipByte('[') {
			c.skipSpaces()
			type state struct {
				sub []predicate
				and bool
			}
			var stack []state
			var sub []predicate
			var and bool
		NextPred:
			if c.skipByte('(') {
				stack = append(stack, state{sub, and})
				sub = nil
				and = false
			}
			var next predicate
			if pos, ok := c.parseInt(); ok {
				if pos == 0 {
					return nil, c.errorf("positions start at 1")
				}
				next = positionPredicate{pos}
			} else if c.skipString("contains(") {
				path, err := c.parsePath()
				if err != nil {
					return nil, err
				}
				c.skipSpaces()
				if !c.skipByte(',') {
					return nil, c.errorf("contains() expected ',' followed by a literal string")
				}
				c.skipSpaces()
				value, err := c.parseLiteral()
				if err != nil {
					return nil, err
				}
				c.skipSpaces()
				if !c.skipByte(')') {
					return nil, c.errorf("contains() missing ')'")
				}
				next = containsPredicate{path, value}
			} else if c.skipString("not(") {
				// TODO Generalize to handle any predicate expression.
				path, err := c.parsePath()
				if err != nil {
					return nil, err
				}
				c.skipSpaces()
				if !c.skipByte(')') {
					return nil, c.errorf("not() missing ')'")
				}
				next = notPredicate{path}
			} else {
				path, err := c.parsePath()
				if err != nil {
					return nil, err
				}
				if path.path[0] == '-' {
					if _, err = strconv.Atoi(path.path); err == nil {
						return nil, c.errorf("positions must be positive")
					}
				}
				c.skipSpaces()
				if c.skipByte('=') {
					c.skipSpaces()
					value, err := c.parseLiteral()
					if err != nil {
						return nil, c.errorf("%v", err)
					}
					next = equalsPredicate{path, value}
				} else {
					next = existsPredicate{path}
				}
			}
		HandleNext:
			if and {
				p := sub[len(sub)-1].(andPredicate)
				p.sub = append(p.sub, next)
				sub[len(sub)-1] = p
			} else {
				sub = append(sub, next)
			}
			if c.skipSpaces() {
				mark := c.i
				if c.skipString("and") && c.skipSpaces() {
					if !and {
						and = true
						sub[len(sub)-1] = andPredicate{[]predicate{sub[len(sub)-1]}}
					}
					goto NextPred
				} else if c.skipString("or") && c.skipSpaces() {
					and = false
					goto NextPred
				} else {
					c.i = mark
				}
			}
			if c.skipByte(')') {
				if len(stack) == 0 {
					return nil, c.errorf("unexpected ')'")
				}
				if len(sub) == 1 {
					next = sub[0]
				} else {
					next = orPredicate{sub}
				}
				s := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				sub = s.sub
				and = s.and
				goto HandleNext
			}
			if len(stack) > 0 {
				return nil, c.errorf("expected ')'")
			}
			if len(sub) == 1 {
				step.pred = sub[0]
			} else {
				step.pred = orPredicate{sub}
			}
			if !c.skipByte(']') {
				return nil, c.errorf("expected ']'")
			}
			c.skipSpaces()
		}
		steps = append(steps, step)
		//fmt.Printf("step: %#v\n", step)
		if !c.skipByte('/') {
			if (start == 0 || start == c.i) && c.i < len(c.path) {
				return nil, c.errorf("unexpected %q", c.path[c.i])
			}
			return &Path{steps: steps, path: c.path[start:c.i]}, nil
		}
	}
	panic("unreachable")
}