/*
 *    Copyright 2025 Jeff Galyan
 *
 *    Licensed under the Apache License, Version 2.0 (the "License");
 *    you may not use this file except in compliance with the License.
 *    You may obtain a copy of the License at
 *
 *        http://www.apache.org/licenses/LICENSE-2.0
 *
 *    Unless required by applicable law or agreed to in writing, software
 *    distributed under the License is distributed on an "AS IS" BASIS,
 *    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *    See the License for the specific language governing permissions and
 *    limitations under the License.
 */

package quokka

import (
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strconv"
)

// BindQuery binds URL query parameters into a struct using `query` struct tags.
// The destination must be a pointer to a struct.
func (c *Context) BindQuery(dst any) error {
	return bindValues(c.R.URL.Query(), dst, "query")
}

// BindForm parses the request form and binds values into a struct using `form`
// struct tags. The destination must be a pointer to a struct.
func (c *Context) BindForm(dst any) error {
	if err := c.R.ParseForm(); err != nil {
		return err
	}
	return bindValues(c.R.Form, dst, "form")
}

func bindValues(vals url.Values, dst any, tagKey string) error {
	rv := reflect.ValueOf(dst)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return errors.New("quokka: bind destination must be a non-nil pointer to a struct")
	}
	rv = rv.Elem()
	if rv.Kind() != reflect.Struct {
		return errors.New("quokka: bind destination must be a pointer to a struct")
	}

	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		tag := field.Tag.Get(tagKey)
		if tag == "" || tag == "-" {
			continue
		}
		val := vals.Get(tag)
		if val == "" {
			continue
		}
		if err := setField(rv.Field(i), val); err != nil {
			return fmt.Errorf("quokka: field %s: %w", field.Name, err)
		}
	}
	return nil
}

func setField(fv reflect.Value, val string) error {
	if !fv.CanSet() {
		return nil
	}
	switch fv.Kind() {
	case reflect.String:
		fv.SetString(val)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return err
		}
		fv.SetInt(n)
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return err
		}
		fv.SetFloat(f)
	case reflect.Bool:
		b, err := strconv.ParseBool(val)
		if err != nil {
			return err
		}
		fv.SetBool(b)
	}
	return nil
}
