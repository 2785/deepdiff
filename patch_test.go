package difff

import (
	"encoding/json"
	"reflect"
	"testing"
)

type PatchTestCase struct {
	description  string
	tree, expect interface{}
	patch        []*Delta
}

func TestPatch(t *testing.T) {
	cases := []PatchTestCase{
		{
			"update bool",
			[]interface{}{true},
			[]interface{}{false},
			[]*Delta{&Delta{Type: DTUpdate, DstPath: "/0", DstVal: false}},
		},
		{
			"update number",
			[]interface{}{float64(1)},
			[]interface{}{float64(2)},
			[]*Delta{&Delta{Type: DTUpdate, DstPath: "/0", DstVal: float64(2)}},
		},
		{
			"update nested number",
			map[string]interface{}{"a": []interface{}{float64(1)}},
			map[string]interface{}{"a": []interface{}{float64(2)}},
			[]*Delta{&Delta{Type: DTUpdate, DstPath: "/a/0", DstVal: float64(2)}},
		},
		{
			"update string",
			[]interface{}{"before"},
			[]interface{}{"after"},
			[]*Delta{&Delta{Type: DTUpdate, DstPath: "/0", DstVal: "after"}},
		},
		{
			"insert number to end of array",
			[]interface{}{},
			[]interface{}{float64(1)},
			[]*Delta{&Delta{Type: DTInsert, DstPath: "/0", DstVal: float64(1)}},
		},
		{
			"insert number in slice",
			[]interface{}{float64(0), float64(2)},
			[]interface{}{float64(0), float64(1), float64(2)},
			[]*Delta{&Delta{Type: DTInsert, DstPath: "/1", DstVal: float64(1)}},
		},
		{
			"insert false into object",
			map[string]interface{}{},
			map[string]interface{}{"a": false},
			[]*Delta{&Delta{Type: DTInsert, DstPath: "/a", DstVal: false}},
		},
		{
			"delete from end of array",
			[]interface{}{"a", "b", "c"},
			[]interface{}{"a", "b"},
			[]*Delta{&Delta{Type: DTDelete, SrcPath: "/2"}},
		},
		{
			"delete from array",
			[]interface{}{"a", "b", "c"},
			[]interface{}{"a", "c"},
			[]*Delta{&Delta{Type: DTDelete, SrcPath: "/1"}},
		},
		{
			"delete from object",
			map[string]interface{}{"a": false},
			map[string]interface{}{},
			[]*Delta{&Delta{Type: DTDelete, SrcPath: "/a"}},
		},
		{
			"delete from nested object",
			map[string]interface{}{
				"a": []interface{}{
					map[string]interface{}{
						"b": false,
					},
				},
			},
			map[string]interface{}{
				"a": []interface{}{
					map[string]interface{}{},
				},
			},
			[]*Delta{&Delta{Type: DTDelete, SrcPath: "/a/0/b"}},
		},
		{
			"move in object",
			map[string]interface{}{"a": false},
			map[string]interface{}{"b": false},
			[]*Delta{&Delta{Type: DTMove, SrcPath: "/a", DstPath: "/b", DstVal: false}},
		},
		{
			"move from object to nested object",
			map[string]interface{}{"a": false, "b": map[string]interface{}{"c": float64(2)}},
			map[string]interface{}{"b": map[string]interface{}{"c": float64(2), "d": false}},
			[]*Delta{&Delta{Type: DTMove, SrcPath: "/a", DstPath: "/b/d", DstVal: false}},
		},
		{
			"insert, update, then delete",
			map[string]interface{}{"a": true, "b": float64(2)},
			map[string]interface{}{"a": false, "c": float64(3)},
			[]*Delta{
				&Delta{Type: DTInsert, DstPath: "/c", DstVal: float64(3)},
				&Delta{Type: DTUpdate, DstPath: "/a", DstVal: false},
				&Delta{Type: DTDelete, SrcPath: "/b", DstVal: false},
			},
		},
		// {
		//  TODO (b5): I have no idea why this isn't working at the moment, need to figure out what's
		//  causing weird refelction pointer nonsense. I think it's from successive delete-then-insert
		// 	"move from object to array",
		// 	map[string]interface{}{"a": false, "b": []interface{}{float64(2)}},
		// 	map[string]interface{}{"b": []interface{}{float64(2), false}},
		// 	[]*Delta{&Delta{Type: DTMove, SrcPath: "/a", DstPath: "/b/1", DstVal: false}},
		// },
		// {
		// 	"move from array to object",
		// 	[]interface{}{float64(32), map[string]interface{}{}},
		// 	[]interface{}{map[string]interface{}{"a": float64(32)}},
		// 	[]*Delta{&Delta{Type: DTMove, SrcPath: "/0", DstPath: "/0/a", DstVal: float64(32)}},
		// },
	}

	for i, c := range cases {
		if err := Patch(&c.tree, c.patch); err != nil {
			t.Errorf("%d. %s error: %s", i, c.description, err)
			continue
		}

		if !reflect.DeepEqual(c.tree, c.expect) {
			t.Errorf("%d. %s result mismatch", i, c.description)
			if data, err := json.Marshal(c.tree); err == nil {
				t.Log("got   :", string(data))
			}
			if data, err := json.Marshal(c.expect); err == nil {
				t.Log("expect:", string(data))
			}
		}
	}
}

type PatchErrorTestCase struct {
	description string
	tree        interface{}
	dlt         *Delta
	err         error
}

func RunPatchErrorTestCases(t *testing.T, cases []PatchErrorTestCase) {

}

func TestPatchErrors(t *testing.T) {
	errCases := []PatchErrorTestCase{}

	RunPatchErrorTestCases(t, errCases)
}