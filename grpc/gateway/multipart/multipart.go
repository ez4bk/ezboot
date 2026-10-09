package multipart

import (
	"bytes"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/url"
	"reflect"
	"strconv"
	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/pkg/errors"
)

const (
	// MIME 表示multipart的MIME类型
	MIME = "multipart/form-data"
)

// Marshaler 表示一个multipart/form-data类型的解析器
type Marshaler struct {
	runtime.JSONBuiltin

	MemoryLimit int64
}

// NewDecoder 创建一个解码器用于解析multipart/form-data类型的请求
func (m *Marshaler) NewDecoder(r io.Reader) runtime.Decoder {
	return &Decoder{r: r, mem: m.MemoryLimit}
}

// Decoder 用于解析multipart/form-data类型的请求
type Decoder struct {
	r      io.Reader
	mem    int64
	form   *multipart.Form
	values url.Values
}

// ParseForm 解析请求体的内容生成 *multipart.Form 对象
func (d *Decoder) ParseForm() (*multipart.Form, error) {
	data, err := ioutil.ReadAll(d.r)
	if err != nil {
		return nil, errors.Wrap(err, "multipart")
	}

	l, r := len(data)-5, len(data)-4 // trim "--\r\n"
	for l >= 0 && data[l] != '\n' {
		l--
	}

	// trim "\n--"
	return multipart.NewReader(bytes.NewReader(data), string(data[l+3:r])).ReadForm(d.mem)
}

// Decode 解析multipart/form-data请求, 并将请求字段填入v中
func (d *Decoder) Decode(v interface{}) (err error) {
	if d.form, err = d.ParseForm(); err != nil {
		return err
	}

	d.values = d.form.Value
	return d.decode(v)
}

// decode 将解析和的form对象填到对象v中
func (d *Decoder) decode(v interface{}) error {
	rv := reflect.Indirect(reflect.ValueOf(v))
	if rv.Kind() != reflect.Struct {
		return errors.New("multipart: invalid value to decode")
	}

	rt := rv.Type()
	for i := 0; i < rv.NumField(); i++ {
		ft, fv := rt.Field(i), rv.Field(i)
		if fv.CanSet() && fv.IsValid() {
			val := d.GetFormValue(ft.Tag)
			switch ft.Type.Kind() {
			case reflect.String:
				fv.Set(reflect.ValueOf(string(val)))
			case reflect.Int32:
				if n, err := strconv.Atoi(string(val)); err == nil {
					fv.Set(reflect.ValueOf(int32(n)))
				}
			case reflect.Int64:
				if n, err := strconv.Atoi(string(val)); err == nil {
					fv.Set(reflect.ValueOf(int64(n)))
				}
			case reflect.Uint32:
				if n, err := strconv.Atoi(string(val)); err == nil {
					fv.Set(reflect.ValueOf(uint32(n)))
				}
			case reflect.Uint64:
				if n, err := strconv.Atoi(string(val)); err == nil {
					fv.Set(reflect.ValueOf(uint64(n)))
				}
			case reflect.Bool:
				b := strings.ToLower(string(val)) == "true"
				fv.Set(reflect.ValueOf(b))
			case reflect.Slice:
				el := ft.Type.Elem()
				switch el.Kind() {
				case reflect.Uint8:
					fv.Set(reflect.ValueOf(val))
				}
			}
		}
	}

	return nil
}

// GetFormValue 从form-data中获取指定的数据并返回
func (d *Decoder) GetFormValue(tag reflect.StructTag) []byte {
	jsonTag := tag.Get("json")
	if data := d.getFormValueJSONTag(jsonTag); data != nil {
		return data
	}

	pbTag := tag.Get("protobuf")
	return d.getFormValueProtobufTag(pbTag)
}

func (d *Decoder) getFormStream(key string) []byte {
	if files, ok := d.form.File[key]; ok && len(files) != 0 {
		if fp, err := files[0].Open(); err == nil {
			defer func() { _ = fp.Close() }()
			if content, err := ioutil.ReadAll(fp); err == nil {
				return content
			}
		}
	}
	return nil
}

func (d *Decoder) getFormValueJSONTag(jsonTag string) []byte {
	key := strings.Split(jsonTag, ",")[0]
	if d.values.Has(key) {
		return []byte(d.values.Get(key))
	}

	return d.getFormStream(key)
}

func (d *Decoder) getFormValueProtobufTag(pbTag string) []byte {
	tags := strings.Split(pbTag, ",")
	for _, v := range tags {
		if strings.HasPrefix(v, "json=") {
			key := v[5:]
			if d.values.Has(key) {
				return []byte(d.values.Get(key))
			}
			return d.getFormStream(key)
		}
	}
	return nil
}
