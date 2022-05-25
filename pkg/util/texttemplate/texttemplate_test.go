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
	"testing"
)

func TestNewFailed(t *testing.T) {
	_, err := New("", "", "", []string{})

	if err == nil {
		t.Error("new template should not succ")
	}
}

func TestNewFailedAtBuild(t *testing.T) {
	_, err := New(DefaultBeginToken, DefaultEndToken, DefaultSeparator, []string{
		"filter.xx.",
	})
	if err == nil {
		t.Error("new template should not succ")
	}
}

func TestSetDictFailed(t *testing.T) {
	tt, err := NewDefault([]string{
		"filter.{}.req.path",
		"filter.{}.req.method",
		"filter.{}.req.body",
		"filter.{}.req.scheme",
		"filter.{}.req.proto",
		"filter.{}.req.host",
		"filter.{}.req.body.{gjson}",
		"filter.{}.req.header.{}",
		"filter.{}.rsp.statuscode",
		"filter.{}.rsp.body.{gjson}",
	})
	if err != nil {
		t.Errorf("new engine failed err %v", err)
	}

	if err = tt.SetDict("no.abc.req.body", "kkk"); err == nil {
		t.Errorf("set dict should failed")
	}
}

func TestNewDefaultTextTemplateFailed(t *testing.T) {
	tt, err := NewDefault([]string{
		"filter.{}.req.path",
		"filter.{}.req.method",
		"filter.{}.req.body",
		"filter.{}.req.scheme",
		"filter.{}.req.proto",
		"filter.{}.req.host",
		"filter.{}.req.body.{gjson}",
		"filter.{}.req.header.{}",
		"filter.{}.rsp.statuscode",
		"filter.{}.rsp.body.{gjson}",
	})
	if err != nil {
		t.Errorf("new engine failed err %v", err)
	}

	if err = tt.SetDict("filter.abc.req.body", "kkk"); err != nil {
		t.Errorf("set failed err =%v", err)
	}

	if s, _ := tt.Render("xxx-[[filter.abc.rsp.body]]--yyy"); s == "xxx-kkk--yyy" {
		t.Errorf("rendering should fail , but succ")
	}
}

func TestNewDefaultTextTemplateNothing(t *testing.T) {
	tt, err := NewDefault([]string{
		"filter.{}.req.path",
		"filter.{}.req.method",
		"filter.{}.req.body",
		"filter.{}.req.scheme",
		"filter.{}.req.proto",
		"filter.{}.req.host",
		"filter.{}.req.body.{gjson}",
		"filter.{}.req.header.{}",
		"filter.{}.rsp.statuscode",
		"filter.{}.rsp.body.{gjson}",
	})
	if err != nil {
		t.Errorf("new engine failed err %v", err)
	}

	if res := tt.MatchMetaTemplate("filter.abc.req.body"); len(res) == 0 {
		t.Errorf("input %s match template %s", "filter.abc.req.body", res)
	}

	if err = tt.SetDict("filter.abc.req.body", "kkk"); err != nil {
		t.Errorf("set failed err =%v", err)
	}

	if s, err := tt.Render("xxx---yyy"); s != "xxx---yyy" || err != nil {
		t.Errorf("rendering fail , result is %s expect xxx---yyy", s)
	}
}

func TestNewDefaultTextTemplateSucc(t *testing.T) {
	tt, err := NewDefault([]string{
		"filter.{}.req.path",
		"filter.{}.req.method",
		"filter.{}.req.body",
		"filter.{}.req.scheme",
		"filter.{}.req.proto",
		"filter.{}.req.host",
		"filter.{}.req.body.{gjson}",
		"filter.{}.req.header.{}",
		"filter.{}.rsp.statuscode",
		"filter.{}.rsp.body.{gjson}",
	})
	if err != nil {
		t.Errorf("new engine failed err %v", err)
	}

	if res := tt.MatchMetaTemplate("filter.abc.req.body"); len(res) == 0 {
		t.Errorf("input %s match template %s", "filter.abc.req.body", res)
	}

	if err = tt.SetDict("filter.abc.req.body", "kkk"); err != nil {
		t.Errorf("set failed err =%v", err)
	}

	if s, err := tt.Render("xxx-[[filter.abc.req.body]]--yyy"); s != "xxx-kkk--yyy" || err != nil {
		t.Errorf("rendering fail , result is %s expect xxx-kkk--yyy", s)
	}

	dict := tt.GetDict()
	if len(dict) == 0 {
		t.Error("get dict failed, should not empty")
	}
}

func TestNewDefaultTextTemplateEmpty(t *testing.T) {
	tt, err := NewDefault([]string{
		"filter.{}.req.path",
		"filter.{}.req.method",
		"filter.{}.req.body",
		"filter.{}.req.scheme",
		"filter.{}.req.proto",
		"filter.{}.req.host",
		"filter.{}.req.body.{gjson}",
		"filter.{}.req.header.{}",
		"filter.{}.rsp.statuscode",
		"filter.{}.rsp.body.{gjson}",
	})
	if err != nil {
		t.Errorf("new engine failed err %v", err)
	}

	if s, _ := tt.Render("xxx-[[filter.abc.req.body]]--yyy"); s != "xxx---yyy" {
		t.Errorf("rendering fail , result is %s expect xxx---yyy", s)
	}
}

func TestDummyTemplate_render(t *testing.T) {
	tt := NewDummyTemplate()
	if err := tt.SetDict("nothing", "nothing"); err != nil {
		t.Errorf("dummy template setdict failed: %v", err)
	}

	if _, err := tt.Render("nonthing to render"); err != nil {
		t.Errorf("dummy template render failed: %v", err)
	}
}

func TestDummyTemplate_setdict(t *testing.T) {
	tt := NewDummyTemplate()
	if err := tt.SetDict("nothing", "nothing"); err != nil {
		t.Errorf("dummy template setdict failed: %v", err)
	}
}

func TestDummyTemplate_getdict(t *testing.T) {
	tt := NewDummyTemplate()
	dict := tt.GetDict()

	if len(dict) != 0 {
		t.Errorf("dummy template extract template rule map with none empty dict: %v", dict)
	}
}

func TestDummyTemplate_matchmetatemplate(t *testing.T) {
	tt := NewDummyTemplate()

	if tmp := tt.MatchMetaTemplate("nothing"); len(tmp) != 0 {
		t.Errorf("dummy template extract template rule map with none empty template: %v", tmp)
	}
}

func TestDummyTemplate_hastemplate(t *testing.T) {
	tt := NewDummyTemplate()

	if tt.HasTemplates("nothing") {
		t.Error("dummy template doesn't need to has template")
	}
}

func TestDummyTemplate_extracttemplaterulemap(t *testing.T) {
	tt := NewDummyTemplate()
	if err := tt.SetDict("nothing", "nothing"); err != nil {
		t.Errorf("dummy template setdict failed: %v", err)
	}

	rule := tt.ExtractTemplateRuleMap("nothing")

	if len(rule) != 0 {
		t.Errorf("dummy template extract template rule map with none empty : %v", rule)
	}
}

func TestDummyTemplate_extractrawtemplateuulemap(t *testing.T) {
	tt := NewDummyTemplate()
	if err := tt.SetDict("nothing", "nothing"); err != nil {
		t.Errorf("dummy template setdict failed: %v", err)
	}

	rule := tt.ExtractRawTemplateRuleMap("nothing")

	if len(rule) != 0 {
		t.Errorf("dummy template extract raw template rule map with none empty : %v", rule)
	}
}

func TestDummyTemplate_newdummytemplate(t *testing.T) {
	tt := NewDummyTemplate()

	if tt == nil {
		t.Errorf("dummy template new failed")
	}
}

func TestTextTemplate_render(t *testing.T) {
	tt, err := New(DefaultBeginToken, DefaultEndToken, DefaultSeparator, []string{
		"filter.{}.req.path",
		"filter.{}.req.method",
		"filter.{}.req.body",
		"filter.{}.req.scheme",
		"filter.{}.req.proto",
		"filter.{}.req.host",
		"filter.{}.req.body.{gjson}",
		"filter.{}.req.header.{}",
		"filter.{}.rsp.statuscode",
		"filter.{}.rsp.body.{gjson}",
	})
	if err != nil {
		t.Errorf("new engine failed err %v", err)
	}

	if res := tt.MatchMetaTemplate("filter.abc.req.body"); len(res) == 0 {
		t.Errorf("input %s match template %s", "filter.abc.req.body", res)
	}

	if err = tt.SetDict("filter.abc.req.body", "kkk"); err != nil {
		t.Errorf("set failed err =%v", err)
	}

	if s, err := tt.Render("xxx-[[filter.abc.req.body]]--yyy"); s != "xxx-kkk--yyy" || err != nil {
		t.Errorf("rendering fail , result is %s expect xxx-kkk--yyy", s)
	}
}

func TestNewTextTemplateRenderGJSON(t *testing.T) {
	tt, err := NewDefault([]string{
		"filter.{}.req.path",
		"filter.{}.req.method",
		"filter.{}.req.body",
		"filter.{}.req.body.{gjson}",
		"filter.{}.req.header.{}",
	})
	if err != nil {
		t.Errorf("new engine failed err %v", err)
	}

	if err = tt.SetDict("filter.abc.req.body", "{\"aaa\":\"bbb\", \"kkk\":\"hhh\", \"t\":{\"jjj\":\"qqq\"}}"); err != nil {
		t.Errorf("set failed err %v", err)
	}

	input := "001010-[[filter.abc.req.body.aaa]]--02020"
	expect := "001010-bbb--02020"

	if s, err := tt.Render(input); s != expect || err != nil {
		t.Errorf("rendering failed err %v", err)
		t.Errorf("dict is %v ", tt.GetDict())
		t.Fatalf("input %s, expect %s , after rending %s ", input, expect, s)
	}

	input = "001010-[[filter.abc.req.body.kkk]]--02020"
	expect = "001010-hhh--02020"

	if s, err := tt.Render(input); s != expect || err != nil {
		t.Errorf("rendering failed err %v", err)
		t.Errorf("dict is %v ", tt.GetDict())
		t.Fatalf("input %s, expect %s , after rending %s ", input, expect, s)

	}

	input = "001010-[[filter.abc.req.body.t.jjj]]--02020"
	expect = "001010-qqq--02020"

	if s, err := tt.Render(input); s != expect || err != nil {
		t.Errorf("rendering failed err %v", err)
		t.Errorf("dict is %v ", tt.GetDict())
		t.Fatalf("input %s, expect %s , after rending %s ", input, expect, s)
	}
}

func TestNewTextTemplateRenderWildCard(t *testing.T) {
	tt, err := NewDefault([]string{
		"filter.{}.req.path",
		"filter.{}.req.method",
		"filter.{}.req.body",
		"filter.{}.req.body.{gjson}",
		"filter.{}.req.header.{}",
	})
	if err != nil {
		t.Errorf("new engine failed err %v", err)
	}

	if err := tt.SetDict("filter.abc.req.header.X-forward", "a.b.com"); err != nil {
		t.Errorf("set failed err =%v", err)
	}

	if err = tt.SetDict("filter.abc.req.header.Y-forward", "q.m.com"); err != nil {
		t.Errorf("set failed err =%v", err)
	}

	input := "001010-[[filter.abc.req.header.X-forward]]--02020"
	expect := "001010-a.b.com--02020"

	if s, err := tt.Render(input); s != expect || err != nil {
		t.Errorf("rendering err is %v ", err)
		t.Errorf("dict is %v ", tt.GetDict())
		t.Fatalf("input %s, expect %s , after rending %s ", input, expect, s)
	}

	input = "001010-[[filter.abc.req.header.Y-forward]]--02020"
	expect = "001010-q.m.com--02020"

	if s, err := tt.Render(input); s != expect || err != nil {
		t.Errorf("rendering err is %v ", err)
		t.Errorf("dict is %v ", tt.GetDict())
		t.Fatalf("input %s, expect %s , after rending %s ", input, expect, s)

	}
}

func TestNewTextTemplateErrGJSON(t *testing.T) {
	tt, err := NewDefault([]string{
		"filter.{}.req.header.{}",
		"filter.{}.rsp.statuscode",
		"filter.{}.req.{gjson}.body",
	})

	if err == nil {
		t.Fatalf("new engine should failed, but succ %v, tt %v", err, tt)
	}
}

func TestNewTextTemplateErrGJSONBegin(t *testing.T) {
	tt, err := NewDefault([]string{
		"filter.{}.req.header.{}",
		"filter.{}.rsp.statuscode",
		"{gjson}.filter.{}.req.body",
	})

	t.Logf("New engine invalid, expect err [%v]", err)
	if err == nil {
		t.Fatalf("new engine should failed, but succ %v, tt %v", err, tt)
	}
}

func TestNewTextTemplateErrWidecardConfilct(t *testing.T) {
	tt, err := NewDefault([]string{
		"filter.{}.req.header.{}",
		"filter.{}.rsp.statuscode",
		"filter.abc.req.header",
	})

	t.Logf("New engine invalid, expect err [%v]", err)
	if err == nil {
		t.Fatalf("new engine should failed, but succ %v, tt %v", err, tt)
	}
}

func TestNewTextTemplateErrGJSONMiddle(t *testing.T) {
	tt, err := NewDefault([]string{
		"filter.{}.req.header.{}",
		"filter.{}.rsp.statuscode",
		"filter.{}.rsp.statuscode",
		"filter.{}.{gjson}.header",
	})

	t.Logf("New engine invalid, expect err [%v]", err)
	if err == nil {
		t.Fatalf("new engine should failed, but succ %v, tt %v", err, tt)
	}
}

func TestNewTextTemplateWithEmpty(t *testing.T) {
	tt, err := NewDefault([]string{})

	t.Logf("New engine invalid, expect err [%v]", err)
	if err == nil {
		t.Fatalf("new engine should failed, but succ %v, tt %v", err, tt)
	}
}

func TestNewTextTemplateWithWidecarFirstLevel(t *testing.T) {
	tt, err := NewDefault([]string{
		"filter",
		"name",
		"key",
		"{}",
	})

	t.Logf("New engine invalid, expect err [%v]", err)
	if err == nil {
		t.Fatalf("new engine should failed, but succ %v, tt %v", err, tt)
	}
}

func TestNewTextTemplateWithWidecarLastLevel(t *testing.T) {
	tt, err := NewDefault([]string{
		"filter.req.http",
		"filter.req.name",
		"filter.req.url",
		"filter.req.{}",
	})

	t.Logf("New engine invalid, expect err [%v]", err)
	if err == nil {
		t.Fatalf("new engine should failed, but succ %v, tt %v", err, tt)
	}
}

func TestNewTextTemplateValidate(t *testing.T) {
	tt, err := NewDefault([]string{
		"filter.{}.req.path",
		"filter.{}.req.method",
		"filter.{}.req.body",
		"filter.{}.req.scheme",
		"filter.{}.req.proto",
		"filter.{}.req.host",
		"filter.{}.req.body.{gjson}",
		"filter.{}.req.header.{}",
		"filter.{}.rsp.statuscode",
		"filter.{}.rsp.body.{gjson}",
	})
	if err != nil {
		t.Errorf("new engine failed err %v", err)
	}

	input := "try to ...[[filter.abc.req]]"
	if tt.HasTemplates(input) == false {
		t.Fatalf("expect match template, but failed %s", input)
	}

	input = "try to ...filter.abc.req"
	if tt.HasTemplates(input) == true {
		t.Fatalf("expect not match template, but succ input [%s]", input)
	}

	input = "try to ...[[filter.abc.tkg]]..."
	if tt.HasTemplates(input) == true {
		t.Fatalf("expect not match template, but succ input [%s]", input)
	}

	input = "try to ...[[.]]..."
	if tt.HasTemplates(input) == true {
		t.Fatalf("expect not match template, but succ input [%s]", input)
	}
}

func TestNewTextTemplateMatchTemplate(t *testing.T) {
	tt, err := NewDefault([]string{
		"filter.{}.req.path",
		"filter.{}.req.method",
		"filter.{}.req.body",
		"filter.{}.req.scheme",
		"filter.{}.req.proto",
		"filter.{}.req.host",
		"filter.{}.req.host.{}",
	})
	if err != nil {
		t.Errorf("new engine failed err %v", err)
	}

	input := "."
	if result := tt.MatchMetaTemplate(input); len(result) != 0 {
		t.Fatalf("expect not match template, but succ input [%s]", result)
	}

	input = ".."
	if result := tt.MatchMetaTemplate(input); len(result) != 0 {
		t.Fatalf("expect not match template, but succ input [%s]", result)
	}

	input = ""
	if result := tt.MatchMetaTemplate(input); len(result) != 0 {
		t.Fatalf("expect not match template, but succ input [%s]", result)
	}

	input = "kkk"
	if result := tt.MatchMetaTemplate(input); len(result) != 0 {
		t.Fatalf("expect not match template, but succ input [%s]", result)
	}

	input = "[["
	if result := tt.MatchMetaTemplate(input); len(result) != 0 {
		t.Fatalf("expect not match template, but succ input [%s]", result)
	}

	input = "]]"
	if result := tt.MatchMetaTemplate(input); len(result) != 0 {
		t.Fatalf("expect not match template, but succ input [%s]", result)
	}
	input = "{}"
	if result := tt.MatchMetaTemplate(input); len(result) != 0 {
		t.Fatalf("expect not match template, but succ input [%s]", result)
	}

	input = "{gjson}"
	if result := tt.MatchMetaTemplate(input); len(result) != 0 {
		t.Fatalf("expect not match template, but succ input [%s]", result)
	}

	input = "filter.name.."
	if result := tt.MatchMetaTemplate(input); len(result) != 0 {
		t.Fatalf("expect not match template, but input [%s]", result)
	}

	input = "filter.name.req"
	if result := tt.MatchMetaTemplate(input); len(result) == 0 {
		t.Fatalf("expect match template, but failed input [%s]", result)
	}

	input = "filter.name.req.host"
	if result := tt.MatchMetaTemplate(input); len(result) == 0 {
		t.Fatalf("expect match template, but failed input [%s]", result)
	}

	input = "filter.name.req.host.abb"
	if result := tt.MatchMetaTemplate(input); len(result) == 0 {
		t.Fatalf("expect match template, but failed input [%s]", result)
	}
}

func TestNewTextTemplateExtractTemplateRuleMap(t *testing.T) {
	tt, err := NewDefault([]string{
		"filter.{}.req.path",
		"filter.{}.req.method",
		"filter.{}.req.body",
		"filter.{}.req.scheme",
		"filter.{}.req.proto",
		"filter.{}.req.host",
		"filter.{}.req.body.{gjson}",
		"filter.{}.req.header.{}",
		"filter.{}.rsp.statuscode",
		"filter.{}.rsp.body.{gjson}",
	})
	if err != nil {
		t.Fatalf("new engine failed err %v", err)
	}

	input := "xxx000[[filter.abc.req.path]]xxx,yyy[[filter.0.rsp.statuscode]],fff[[filter.3.req.body.jjj.qqq.ddd]]"

	m := map[string]string{}
	if m = tt.ExtractTemplateRuleMap(input); len(m) == 0 {
		t.Fatalf("extract from input %s faile, nothing matched", input)
	}

	key1 := "filter.abc.req.path"
	key2 := "filter.0.rsp.statuscode"
	key3 := "filter.3.req.body.jjj.qqq.ddd"
	value3 := "filter.3.req.body.{gjson}"

	if k, exist := m[key1]; !exist {
		t.Errorf("extract from key1 :%s from input %s faile, nothing matched", key1, input)
	} else {
		if k != key1 {
			t.Errorf("extract from key1 :%s  value %s from input %s not matched", key1, k, input)
		}
	}

	if k, exist := m[key2]; !exist {
		t.Errorf("extract from key1 :%s from input %s faile, nothing matched", key2, input)
	} else {
		if k != key2 {
			t.Errorf("extract from key1 :%s  value %s from input %s not matched", key2, k, input)
		}
	}

	if k, exist := m[key3]; !exist {
		t.Errorf("extract from key3 :%s from input %s faile, nothing matched", key3, input)
	} else {
		if k != value3 {
			t.Errorf("extract from key3 :%s  value %s from input %s not matched expect %s", key3, k, input, value3)
		}
	}

	input = "[[filter.abc.req.path]]xxx,yyy[[filter.0.rsp.statuscode]],fff[[filter.3.req.body.jjj.qqq.ddd]]"
	if m = tt.ExtractTemplateRuleMap(input); len(m) == 0 {
		t.Fatalf("extract from input %s faile, nothing matched", input)
	}
}

func TestNewTextTemplateExtractTemplateRuleMapEmpty(t *testing.T) {
	tt, err := NewDefault([]string{
		"filter.{}.req.path",
		"filter.{}.req.method",
		"filter.{}.req.body",
		"filter.{}.req.scheme",
		"filter.{}.req.proto",
		"filter.{}.req.host",
		"filter.{}.req.body.{gjson}",
		"filter.{}.req.header.{}",
		"filter.{}.rsp.statuscode",
		"filter.{}.rsp.body.{gjson}",
	})
	if err != nil {
		t.Fatalf("new engine failed err %v", err)
	}

	input := "xxx00011111[["

	m := map[string]string{}
	if m = tt.ExtractTemplateRuleMap(input); len(m) != 0 {
		t.Fatalf("extract from input %s no match expect, should be empty", input)
	}

	input = "[[filter.abc.req[[]"
	if m = tt.ExtractTemplateRuleMap(input); len(m) != 0 {
		t.Fatalf("extract from input %s no match expect, should be empty", input)
	}

	input = "xxxxx[[filter.abc.req]"
	if m = tt.ExtractTemplateRuleMap(input); len(m) != 0 {
		t.Fatalf("extract from input %s no match expect, should be empty", input)
	}

	input = "[[filter.abc.req]"
	if m = tt.ExtractTemplateRuleMap(input); len(m) != 0 {
		t.Fatalf("extract from input %s no match expect, should be empty", input)
	}
}

func TestNewTextTemplateExtractRawTemplateRuleMapEmpty(t *testing.T) {
	tt, err := NewDefault([]string{
		"filter.{}.req.path",
		"filter.{}.req.method",
		"filter.{}.req.body",
		"filter.{}.req.scheme",
		"filter.{}.req.proto",
		"filter.{}.req.host",
		"filter.{}.req.body.{gjson}",
		"filter.{}.req.header.{}",
		"filter.{}.rsp.statuscode",
		"filter.{}.rsp.body.{gjson}",
	})
	if err != nil {
		t.Fatalf("new engine failed err %v", err)
	}

	input := "xxx00011111[["

	m := map[string]string{}
	if m = tt.ExtractRawTemplateRuleMap(input); len(m) != 0 {
		t.Fatalf("extract from input %s no match expect, should be empty", input)
	}

	input = "[[filter.abc.req[[]"
	if m = tt.ExtractRawTemplateRuleMap(input); len(m) != 0 {
		t.Fatalf("extract from input %s no match expect, should be empty", input)
	}

	input = "xxxxx[[filter.abc.req]"
	if m = tt.ExtractRawTemplateRuleMap(input); len(m) != 0 {
		t.Fatalf("extract from input %s no match expect, should be empty", input)
	}

	input = "[[filter.abc.req]"
	if m = tt.ExtractRawTemplateRuleMap(input); len(m) != 0 {
		t.Fatalf("extract from input %s no match expect, should be empty", input)
	}

	input = "[[filter.abc.req]]"
	if m = tt.ExtractRawTemplateRuleMap(input); len(m) == 0 {
		t.Fatalf("extract from input %s no match expect, should not be empty", input)
	}

	input = "[[filter.abc.red]]"
	if m = tt.ExtractRawTemplateRuleMap(input); len(m) == 0 {
		t.Fatalf("extract from input %s no match expect, should not be empty", input)
	}

	input = "[[filter.abc.red]] -- [[filter.abc.req.host]]"
	if m = tt.ExtractRawTemplateRuleMap(input); len(m) != 2 {
		t.Fatalf("extract from input %s no match expect, should extract two target", input)
	}

	input = "[[filter.abc.red]] -- [[filter.abc.req.nono]] -- [[filter.abc.rsp.yes]]!!"
	if m = tt.ExtractRawTemplateRuleMap(input); len(m) != 3 {
		t.Fatalf("extract from input %s no match expect, should extract two target", input)
	}
}
