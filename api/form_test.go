package api

import "testing"

func TestToFormData(t *testing.T) {
	type args struct {
		data any
	}
	type Address struct {
		Name  string
		Phone string
	}
	type User struct {
		Name        string                       `form:"name"`
		Age         uint8                        `form:"age"`
		Gender      string                       `form:"gender"`
		Address     []Address                    `form:"address"`
		Active      bool                         `form:"active"`
		MapExample  map[string]string            `form:"map_example"`
		NestedMap   map[string]map[string]string `form:"nested_map"`
		NestedArray [][]string                   `form:"nested_array"`
	}
	tests := []struct {
		name    string
		args    args
		wantRes string
	}{
		{
			args: args{User{
				Name:   "joeybloggs",
				Age:    3,
				Gender: "Male",
				Address: []Address{
					{Name: "26 Here Blvd.", Phone: "9(999)999-9999"},
					{Name: "26 There Blvd.", Phone: "1(111)111-1111"},
				},
				Active:      true,
				MapExample:  map[string]string{"key": "value"},
				NestedMap:   map[string]map[string]string{"key": {"key": "value"}},
				NestedArray: [][]string{{"value"}},
			}},
			wantRes: `
				address[0].Phone:9(999)999-9999
				address[1].Phone:1(111)111-1111
				address[1].Name:26 There Blvd.
				gender:Male
				age:3
				nested_array[0][0]:value
				nested_map[key][key]:value
				address[0].Name:26 Here Blvd.
				name:joeybloggs
				map_example[key]:value
				active:true
			`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotRes := ToFormData(&tt.args.data); gotRes != tt.wantRes {
				t.Logf("ToFormData() = %v, want %v", gotRes, tt.wantRes)
			}
		})
	}
}
