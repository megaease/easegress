/*
 * Copyright (c) 2017, MegaEase
 * All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package texttemplate

import (
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/valyala/fasttemplate"
)

// The complete format of template sentence  is
// ${beginToken}${tag1}${separator}${tag2}${separator}...${endtoken}
// e.g., if beginToken is '[[', endtoken is ']]', separator is '.'
// [[filter.{}.req.body.{gjson}]]
// [[filter.{}.req.body.{gjson}]]
// TemplateEngine is the abstract implementer
// Template is the part of user's input string's which we want the TemplateEngine to render it
// MetaTemplate is the description collections for TemplateEngine to identify user's valid template rules
const (
	// regexpSyntax = "\\[\\[(.*?)\\]\\]"

	// WidecardTag means accepting any none empty string
	// if chose "{}" to accept any none empty string, then should provide another tag value at that level
	WidecardTag = "{}"

	// GJSONTag is the special hardcode tag for indicating GJSON syntax, must appear in the last
	// of one template, if chose "{GJSON}", should provide another tag value at that level
	GJSONTag = "{gjson}"

	DefaultBeginToken = "[["
	DefaultEndToken   = "]]"
	DefaultSeparator  = "."
)

type node struct {
	Value    string // The tag,e.g. 'filter', 'req'
	Children []*node
}

// TemplateEngine is the basic API collection for a template usage
type TemplateEngine interface {
	// Render Rendering e.g., [[xxx.xx.dd.xx]]'s value is 'value0', [[yyy.www.zzz]]'s value is 'value1'
	// "aaa-[[xxx.xx.dd.xx]]-bbb 10101-[[yyy.wwww.zzz]]-9292" will be rendered to "aaa-value0-bbb 10101-value1-9292"
	// Also support GJSON syntax at last tag
	Render(input string) (string, error)

	// ExtractTemplateRuleMap extracts templates from input string
	// return map's key is the template, the value is the matched and rendered metaTemplate
	ExtractTemplateRuleMap(input string) map[string]string

	// ExtractRawTemplateRuleMap extracts templates from input string
	// return map's key is the template, the value is the matched and rendered metaTemplate or empty
	ExtractRawTemplateRuleMap(input string) map[string]string

	// HasTemplates checks whether it has templates in input string or not
	HasTemplates(input string) bool

	// MatchMetaTemplate return original template or replace with {gjson} at last tag, "" if not metaTemplate matched
	MatchMetaTemplate(template string) string

	// SetDict adds a temaplateRule and its value for later rendering
	SetDict(template string, value interface{}) error

	// GetDict returns the template's dictionary
	GetDict() map[string]interface{}
}

// DummyTemplate return a empty implement
type DummyTemplate struct{}

// Render dummy implement
func (DummyTemplate) Render(input string) (string, error) {
	return "", nil
}

// ExtractTemplateRuleMap dummy implement
func (DummyTemplate) ExtractTemplateRuleMap(input string) map[string]string {
	m := make(map[string]string, 0)
	return m
}

// ExtractRawTemplateRuleMap dummy implement
func (DummyTemplate) ExtractRawTemplateRuleMap(input string) map[string]string {
	m := make(map[string]string, 0)
	return m
}

// SetDict the dummy implement
func (DummyTemplate) SetDict(template string, value interface{}) error {
	return nil
}

// MatchMetaTemplate dummy implement
func (DummyTemplate) MatchMetaTemplate(template string) string {
	return ""
}

// GetDict the dummy implement
func (DummyTemplate) GetDict() map[string]interface{} {
	m := make(map[string]interface{}, 0)
	return m
}

// HasTemplates the dummy implement
func (DummyTemplate) HasTemplates(input string) bool {
	return false
}

// TextTemplate wraps a fasttempalte rendering and a
// template syntax tree for validation, the valid template and its
// value can be added into dictionary for rendering
type TextTemplate struct {
	beginToken string
	endToken   string
	separator  string

	metaTemplates []string               // the user raw input candidate templates
	root          *node                  // The template syntax tree root node generated by use's input raw templates
	dict          map[string]interface{} // using `interface{}` for fasttemplate's API
}

// NewDefault returns Template interface implementer with default config and customize meatTemplates
func NewDefault(metaTemplates []string) (TemplateEngine, error) {
	return New(DefaultBeginToken, DefaultEndToken, DefaultSeparator, metaTemplates)
}

// New returns a new Template interface implementer, return a dummy template if something wrong,
// and in that case, the dedicated reason will set into error return
func New(beginToken, endToken, separator string, metaTemplates []string) (TemplateEngine, error) {
	if len(beginToken) == 0 || len(endToken) == 0 || len(separator) == 0 || len(metaTemplates) == 0 {
		format := "invalid parameter: beingToken %s, endToken %s, separator %s, metaTempaltes %v"
		return nil, fmt.Errorf(format, beginToken, endToken, separator, metaTemplates)
	}
	t := &TextTemplate{
		beginToken:    beginToken,
		endToken:      endToken,
		separator:     separator,
		metaTemplates: metaTemplates,
		dict:          map[string]interface{}{},
	}

	if err := t.buildTemplateTree(); err != nil {
		return nil, err
	}

	return t, nil
}

// NewDummyTemplate returns a dummy template implement
func NewDummyTemplate() TemplateEngine {
	return DummyTemplate{}
}

// GetDict return the dictionary of texttemplate
func (t *TextTemplate) GetDict() map[string]interface{} {
	return t.dict
}

func (t *TextTemplate) indexChild(children []*node, target string) int {
	for i, v := range children {
		if target == v.Value {
			return i
		}
	}
	return -1
}

func (t *TextTemplate) addNode(tags []string) {
	parent := t.root
	for _, v := range tags {
		index := t.indexChild(parent.Children, v)

		if index != -1 {
			parent = parent.Children[index]
			continue
		}

		tmp := &node{Value: v}
		parent.Children = append(parent.Children, tmp)
		parent = tmp
	}
}

func (t *TextTemplate) validateTree(root *node) error {
	if len(root.Children) == 0 {
		return nil
	}

	if len(root.Children) == 1 {
		return t.validateTree(root.Children[0])
	}

	if index := t.indexChild(root.Children, WidecardTag); index != -1 {
		return fmt.Errorf("{} wildcard and other tags exist at the same level")
	}

	if index := t.indexChild(root.Children, GJSONTag); index != -1 {
		return fmt.Errorf("{gjson} GJSON and other tags exist at the same level")
	}

	for _, child := range root.Children {
		if err := t.validateTree(child); err != nil {
			return err
		}
	}

	return nil
}

func (t *TextTemplate) buildTemplateTree() error {
	t.root = &node{}

	for _, v := range t.metaTemplates {
		tags := strings.Split(v, t.separator)

		for i, tag := range tags {
			if len(tag) == 0 {
				format := "invalid empty tag, template %s index %d seprator %s"
				return fmt.Errorf(format, v, i, t.separator)
			}

			if tag == GJSONTag && i != len(tags)-1 {
				format := "invalid %s: GJSON tag should only appear at the end"
				return fmt.Errorf(format, v)
			}
		}

		t.addNode(tags)
	}

	// validate the whole template tree
	if err := t.validateTree(t.root); err != nil {
		t.root = nil
		return fmt.Errorf("invalid templates %v, err is %v ", t.metaTemplates, err)
	}

	return nil
}

// MatchMetaTemplate travels the metaTemplate syntax tree and return the first match template
// if matched found
//   e.g. template is "filter.abc.req.body.friends.#(last=="Murphy").first" match "filter.{}.req.body.{gjson}"
//   	will return "filter.abc.req.body.{gjson}"
//   e.g. template is "filter.abc.req.body" match "filter.{}.req.body"
//   	will return "filter.abc.req.body"
// if not any template matched found, then return ""
func (t *TextTemplate) MatchMetaTemplate(template string) string {
	tags := strings.Split(template, t.separator)
	if len(tags) == 0 {
		return ""
	}

	root := t.root
	index := 0
	hasGJSON := false

	for ; index < len(tags); index++ {
		// no tag remain to match, or it's an empty tag
		if len(root.Children) == 0 || len(tags[index]) == 0 {
			return ""
		}

		if len(root.Children) == 1 {
			if root.Children[0].Value == GJSONTag {
				hasGJSON = true
				break
			}
			if root.Children[0].Value == WidecardTag || root.Children[0].Value == tags[index] {
				root = root.Children[0]
				continue
			} else {
				return ""
			}
		} else {
			if index := t.indexChild(root.Children, tags[index]); index != -1 {
				root = root.Children[index]
			} else {
				// no match at current level, return fail directly
				return ""
			}
		}
	}

	if hasGJSON {
		// replace left gjson syntax with GJSONTag
		return strings.Join(tags[:index], t.separator) + t.separator + GJSONTag
	}

	return template
}

func (t *TextTemplate) extractVarsAroundToken(input string, varFunc func(v string) bool) {
	for len(input) != 0 {
		idx := strings.Index(input, t.beginToken)
		if idx == -1 {
			break
		}

		input = input[idx+len(t.beginToken):] // jump over the beginning token

		idx = strings.Index(input, t.endToken)
		if idx == -1 {
			break
		}

		if !varFunc(input[:idx]) {
			break
		}

		input = input[idx+len(t.endToken):]
	}
}

// ExtractTemplateRuleMap extracts candidate templates from input string
// return map's key is the candidate template, the value is the matched template
func (t *TextTemplate) ExtractTemplateRuleMap(input string) map[string]string {
	m := map[string]string{}

	t.extractVarsAroundToken(input, func(v string) bool {
		metaTemplate := t.MatchMetaTemplate(v)
		if len(metaTemplate) != 0 {
			m[v] = metaTemplate
		}
		return true
	})

	return m
}

// ExtractRawTemplateRuleMap extracts all candidate templates (valid/invalid)
// from input string
func (t *TextTemplate) ExtractRawTemplateRuleMap(input string) map[string]string {
	m := map[string]string{}

	t.extractVarsAroundToken(input, func(v string) bool {
		m[v] = t.MatchMetaTemplate(v)
		return true
	})

	return m
}

// SetDict adds a templateRule into dictionary if it contains any templates.
func (t *TextTemplate) SetDict(template string, value interface{}) error {
	if tmp := t.MatchMetaTemplate(template); len(tmp) != 0 {
		t.dict[template] = value
		return nil
	}

	return fmt.Errorf("matched none template , input %s ", template)
}

func (t *TextTemplate) setWithGJSON(template, metaTemplate string) error {
	keyIndict := strings.TrimRight(metaTemplate, t.separator+GJSONTag)
	gjsonSyntax := strings.TrimPrefix(template, keyIndict+t.separator)

	if valueForGJSON, exist := t.dict[keyIndict]; exist {
		if err := t.SetDict(template, gjson.Get(valueForGJSON.(string), gjsonSyntax).String()); err != nil {
			return err
		}
	} else {
		return fmt.Errorf("set gjson found no syntax target, template %s", template)
	}

	return nil
}

// HasTemplates check a string contain any valid templates
func (t *TextTemplate) HasTemplates(input string) bool {
	has := false
	t.extractVarsAroundToken(input, func(v string) bool {
		has = t.MatchMetaTemplate(v) != ""
		return !has
	})
	return has
}

// Render uses a fasttemplate and dictionary to rendering
//  e.g., [[xxx.xx.dd.xx]]'s value in dictionary is 'value0', [[yyy.www.zzz]]'s value is 'value1'
// "aaa-[[xxx.xx.dd.xx]]-bbb 10101-[[yyy.wwww.zzz]]-9292" will be rendered to "aaa-value0-bbb 10101-value1-9292"
// if containers any new GJSON syntax, it will use 'gjson.Get' to extract result then store into dictionary before
// rendering
func (t *TextTemplate) Render(input string) (string, error) {
	var (
		err    error
		hasVar bool
	)

	t.extractVarsAroundToken(input, func(v string) bool {
		meta := t.MatchMetaTemplate(v)
		if len(meta) == 0 {
			return true
		}

		hasVar = true
		if !strings.Contains(meta, GJSONTag) {
			return true
		}

		// has new gjson syntax, add manually
		if _, exist := t.dict[v]; !exist {
			if err = t.setWithGJSON(v, meta); err != nil {
				return false
			}
		}

		return true
	})

	if err != nil {
		return "", err
	}

	if !hasVar {
		return input, nil
	}

	return fasttemplate.ExecuteString(input, t.beginToken, t.endToken, t.dict), nil
}
