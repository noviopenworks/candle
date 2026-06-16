package proto

import (
	"context"
	"fmt"

	"github.com/bufbuild/protocompile"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

// RPC is a normalized gRPC method.
type RPC struct {
	Name            string
	FullName        string
	RequestMessage  string
	ResponseMessage string
	StreamKind      string // unary|server_stream|client_stream|bidi
}

// Service is a normalized gRPC service.
type Service struct {
	Name     string
	FullName string
	RPCs     []RPC
}

// Field is a normalized message field.
type Field struct {
	Name   string
	Type   string
	Number int32
	Label  string
}

// Message is a normalized protobuf message.
type Message struct {
	Name     string
	FullName string
	Fields   []Field
}

// EnumValue is a normalized enum value.
type EnumValue struct {
	Name   string
	Number int32
}

// Enum is a normalized protobuf enum.
type Enum struct {
	Name     string
	FullName string
	Values   []EnumValue
}

// File is a normalized .proto file.
type File struct {
	Path      string
	Package   string
	GoPackage string
	Imports   []string
	Services  []Service
	Messages  []Message
	Enums     []Enum
}

// ParseFiles compiles importPaths-rooted .proto files. Each entry in files is an
// import-relative path. Returns normalized files, non-fatal warnings, and a hard
// error only on unexpected failures (a file that fails to compile is reported as
// a warning, not a hard error).
func ParseFiles(roots, files []string) ([]File, []string, error) {
	if len(files) == 0 {
		return nil, nil, nil
	}
	var warns []string
	compiler := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(&protocompile.SourceResolver{ImportPaths: roots}),
	}
	out := make([]File, 0, len(files))
	for _, rel := range files {
		fds, err := compiler.Compile(context.Background(), rel)
		if err != nil {
			warns = append(warns, fmt.Sprintf("%s: %v", rel, err))
			continue
		}
		for _, fd := range fds {
			out = append(out, normalizeFile(rel, fd))
		}
	}
	return out, warns, nil
}

func normalizeFile(path string, fd protoreflect.FileDescriptor) File {
	f := File{Path: path, Package: string(fd.Package())}
	if opts, ok := fd.Options().(*descriptorpb.FileOptions); ok && opts != nil {
		f.GoPackage = opts.GetGoPackage()
	}
	imps := fd.Imports()
	for i := 0; i < imps.Len(); i++ {
		f.Imports = append(f.Imports, imps.Get(i).Path())
	}
	svcs := fd.Services()
	for i := 0; i < svcs.Len(); i++ {
		f.Services = append(f.Services, normalizeService(svcs.Get(i)))
	}
	// Walk top-level messages and recurse into their nested messages/enums so
	// every type is flattened into File.Messages/File.Enums keyed by its dotted
	// FullName. Nested types include synthetic map-entry messages that map fields
	// generate.
	msgs := fd.Messages()
	for i := 0; i < msgs.Len(); i++ {
		collectMessages(&f, msgs.Get(i))
	}
	ens := fd.Enums()
	for i := 0; i < ens.Len(); i++ {
		f.Enums = append(f.Enums, normalizeEnum(ens.Get(i)))
	}
	return f
}

// collectMessages appends md and all of its nested messages and enums (to
// arbitrary depth) to f.
func collectMessages(f *File, md protoreflect.MessageDescriptor) {
	f.Messages = append(f.Messages, normalizeMessage(md))
	nested := md.Messages()
	for i := 0; i < nested.Len(); i++ {
		collectMessages(f, nested.Get(i))
	}
	ens := md.Enums()
	for i := 0; i < ens.Len(); i++ {
		f.Enums = append(f.Enums, normalizeEnum(ens.Get(i)))
	}
}

func normalizeService(sd protoreflect.ServiceDescriptor) Service {
	s := Service{Name: string(sd.Name()), FullName: string(sd.FullName())}
	ms := sd.Methods()
	for i := 0; i < ms.Len(); i++ {
		m := ms.Get(i)
		s.RPCs = append(s.RPCs, RPC{
			Name:            string(m.Name()),
			FullName:        string(m.FullName()),
			RequestMessage:  string(m.Input().FullName()),
			ResponseMessage: string(m.Output().FullName()),
			StreamKind:      streamKind(m.IsStreamingClient(), m.IsStreamingServer()),
		})
	}
	return s
}

func streamKind(client, server bool) string {
	switch {
	case client && server:
		return "bidi"
	case client:
		return "client_stream"
	case server:
		return "server_stream"
	default:
		return "unary"
	}
}

func normalizeMessage(md protoreflect.MessageDescriptor) Message {
	m := Message{Name: string(md.Name()), FullName: string(md.FullName())}
	fs := md.Fields()
	for i := 0; i < fs.Len(); i++ {
		fd := fs.Get(i)
		m.Fields = append(m.Fields, Field{
			Name:   string(fd.Name()),
			Type:   fieldType(fd),
			Number: int32(fd.Number()),
			Label:  fieldLabel(fd),
		})
	}
	return m
}

func fieldType(fd protoreflect.FieldDescriptor) string {
	if fd.Kind() == protoreflect.MessageKind || fd.Kind() == protoreflect.GroupKind {
		return string(fd.Message().FullName())
	}
	if fd.Kind() == protoreflect.EnumKind {
		return string(fd.Enum().FullName())
	}
	return fd.Kind().String()
}

func fieldLabel(fd protoreflect.FieldDescriptor) string {
	switch {
	case fd.IsList():
		return "repeated"
	case fd.IsMap():
		return "map"
	default:
		return "optional"
	}
}

func normalizeEnum(ed protoreflect.EnumDescriptor) Enum {
	e := Enum{Name: string(ed.Name()), FullName: string(ed.FullName())}
	vs := ed.Values()
	for i := 0; i < vs.Len(); i++ {
		v := vs.Get(i)
		e.Values = append(e.Values, EnumValue{Name: string(v.Name()), Number: int32(v.Number())})
	}
	return e
}
