package api

import (
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/pocketbase/pocketbase/core"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	// Force registration of all whatsmeow proto packages so they appear in the schema browser.
	_ "go.mau.fi/whatsmeow/proto/armadilloutil"
	_ "go.mau.fi/whatsmeow/proto/instamadilloAddMessage"
	_ "go.mau.fi/whatsmeow/proto/instamadilloCoreTypeActionLog"
	_ "go.mau.fi/whatsmeow/proto/instamadilloCoreTypeAdminMessage"
	_ "go.mau.fi/whatsmeow/proto/instamadilloCoreTypeCollection"
	_ "go.mau.fi/whatsmeow/proto/instamadilloCoreTypeLink"
	_ "go.mau.fi/whatsmeow/proto/instamadilloCoreTypeMedia"
	_ "go.mau.fi/whatsmeow/proto/instamadilloCoreTypeText"
	_ "go.mau.fi/whatsmeow/proto/instamadilloDeleteMessage"
	_ "go.mau.fi/whatsmeow/proto/instamadilloSupplementMessage"
	_ "go.mau.fi/whatsmeow/proto/instamadilloTransportPayload"
	_ "go.mau.fi/whatsmeow/proto/instamadilloXmaContentRef"
	_ "go.mau.fi/whatsmeow/proto/waAICommon"
	_ "go.mau.fi/whatsmeow/proto/waAICommonDeprecated"
	_ "go.mau.fi/whatsmeow/proto/waAdv"
	_ "go.mau.fi/whatsmeow/proto/waArmadilloApplication"
	_ "go.mau.fi/whatsmeow/proto/waArmadilloBackupCommon"
	_ "go.mau.fi/whatsmeow/proto/waArmadilloBackupMessage"
	_ "go.mau.fi/whatsmeow/proto/waArmadilloICDC"
	_ "go.mau.fi/whatsmeow/proto/waArmadilloMiTransportAdminMessage"
	_ "go.mau.fi/whatsmeow/proto/waArmadilloTransportEvent"
	_ "go.mau.fi/whatsmeow/proto/waArmadilloXMA"
	_ "go.mau.fi/whatsmeow/proto/waBotMetadata"
	_ "go.mau.fi/whatsmeow/proto/waCert"
	_ "go.mau.fi/whatsmeow/proto/waChatLockSettings"
	_ "go.mau.fi/whatsmeow/proto/waCommon"
	_ "go.mau.fi/whatsmeow/proto/waCommonParameterised"
	_ "go.mau.fi/whatsmeow/proto/waCompanionReg"
	_ "go.mau.fi/whatsmeow/proto/waConsumerApplication"
	_ "go.mau.fi/whatsmeow/proto/waConsumerApplicationParameterised"
	_ "go.mau.fi/whatsmeow/proto/waDeviceCapabilities"
	_ "go.mau.fi/whatsmeow/proto/waE2E"
	_ "go.mau.fi/whatsmeow/proto/waE2EGuest"
	_ "go.mau.fi/whatsmeow/proto/waEphemeral"
	_ "go.mau.fi/whatsmeow/proto/waFingerprint"
	_ "go.mau.fi/whatsmeow/proto/waGroupHistory"
	_ "go.mau.fi/whatsmeow/proto/waHistorySync"
	_ "go.mau.fi/whatsmeow/proto/waLidMigrationSyncPayload"
	_ "go.mau.fi/whatsmeow/proto/waMediaEntryData"
	_ "go.mau.fi/whatsmeow/proto/waMediaTransport"
	_ "go.mau.fi/whatsmeow/proto/waMmsRetry"
	_ "go.mau.fi/whatsmeow/proto/waMsgApplication"
	_ "go.mau.fi/whatsmeow/proto/waMsgTransport"
	_ "go.mau.fi/whatsmeow/proto/waMultiDevice"
	_ "go.mau.fi/whatsmeow/proto/waQuickPromotionSurfaces"
	_ "go.mau.fi/whatsmeow/proto/waReporting"
	_ "go.mau.fi/whatsmeow/proto/waRoutingInfo"
	_ "go.mau.fi/whatsmeow/proto/waServerSync"
	_ "go.mau.fi/whatsmeow/proto/waStatusAttributions"
	_ "go.mau.fi/whatsmeow/proto/waSyncAction"
	_ "go.mau.fi/whatsmeow/proto/waSyncdSnapshotRecovery"
	_ "go.mau.fi/whatsmeow/proto/waUserPassword"
	_ "go.mau.fi/whatsmeow/proto/waVnameCert"
	_ "go.mau.fi/whatsmeow/proto/waWa6"
	_ "go.mau.fi/whatsmeow/proto/waWeb"
	_ "go.mau.fi/whatsmeow/proto/waWinUIApi"
)

// ── schema cache ──────────────────────────────────────────────────────────────

var (
	protoSchemaOnce  sync.Once
	protoSchemaCache *protoSchemaResponse
)

type protoFieldDesc struct {
	Number  int32  `json:"number"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Label   string `json:"label"`
	TypeRef string `json:"type_ref,omitempty"`
	OneOf   string `json:"oneof,omitempty"`
}

type protoEnumValueDesc struct {
	Name   string `json:"name"`
	Number int32  `json:"number"`
}

type protoEnumDesc struct {
	FullName string               `json:"full_name"`
	Package  string               `json:"package"`
	Values   []protoEnumValueDesc `json:"values"`
}

type protoMessageDesc struct {
	FullName string           `json:"full_name"`
	Package  string           `json:"package"`
	Fields   []protoFieldDesc `json:"fields"`
	OneOfs   []string         `json:"oneofs,omitempty"`
	Nested   []string         `json:"nested,omitempty"`
	Enums    []string         `json:"enums,omitempty"`
}

type protoSchemaResponse struct {
	Messages []protoMessageDesc `json:"messages"`
	Enums    []protoEnumDesc    `json:"enums"`
	Packages []string           `json:"packages"`
	Stats    map[string]int     `json:"stats"`
}

func buildProtoSchema() *protoSchemaResponse {
	resp := &protoSchemaResponse{
		Stats: make(map[string]int),
	}
	pkgSet := make(map[string]struct{})

	// Collect messages
	protoregistry.GlobalTypes.RangeMessages(func(mt protoreflect.MessageType) bool {
		md := mt.Descriptor()
		fullName := string(md.FullName())
		pkg := extractProtoPackage(fullName)
		pkgSet[pkg] = struct{}{}

		msg := protoMessageDesc{
			FullName: fullName,
			Package:  pkg,
		}

		// Fields
		fields := md.Fields()
		for i := range fields.Len() {
			fd := fields.Get(i)
			f := protoFieldDesc{
				Number: int32(fd.Number()),
				Name:   string(fd.Name()),
				Type:   fd.Kind().String(),
				Label:  cardinalityLabel(fd.Cardinality()),
			}
			if fd.ContainingOneof() != nil {
				f.OneOf = string(fd.ContainingOneof().Name())
			}
			switch fd.Kind() {
			case protoreflect.MessageKind, protoreflect.GroupKind:
				if fd.Message() != nil {
					f.TypeRef = string(fd.Message().FullName())
				}
			case protoreflect.EnumKind:
				if fd.Enum() != nil {
					f.TypeRef = string(fd.Enum().FullName())
				}
			}
			msg.Fields = append(msg.Fields, f)
		}

		// OneOfs
		oneofs := md.Oneofs()
		for i := range oneofs.Len() {
			msg.OneOfs = append(msg.OneOfs, string(oneofs.Get(i).Name()))
		}

		// Nested messages
		nested := md.Messages()
		for i := range nested.Len() {
			msg.Nested = append(msg.Nested, string(nested.Get(i).FullName()))
		}

		// Nested enums
		enums := md.Enums()
		for i := range enums.Len() {
			msg.Enums = append(msg.Enums, string(enums.Get(i).FullName()))
		}

		resp.Messages = append(resp.Messages, msg)
		return true
	})

	// Collect top-level enums
	protoregistry.GlobalTypes.RangeEnums(func(et protoreflect.EnumType) bool {
		ed := et.Descriptor()
		fullName := string(ed.FullName())
		pkg := extractProtoPackage(fullName)
		pkgSet[pkg] = struct{}{}

		enum := protoEnumDesc{
			FullName: fullName,
			Package:  pkg,
		}
		vals := ed.Values()
		for i := range vals.Len() {
			v := vals.Get(i)
			enum.Values = append(enum.Values, protoEnumValueDesc{
				Name:   string(v.Name()),
				Number: int32(v.Number()),
			})
		}
		resp.Enums = append(resp.Enums, enum)
		return true
	})

	// Sort for deterministic output
	sort.Slice(resp.Messages, func(i, j int) bool {
		return resp.Messages[i].FullName < resp.Messages[j].FullName
	})
	sort.Slice(resp.Enums, func(i, j int) bool {
		return resp.Enums[i].FullName < resp.Enums[j].FullName
	})

	// Packages list
	for p := range pkgSet {
		if p != "" {
			resp.Packages = append(resp.Packages, p)
		}
	}
	sort.Strings(resp.Packages)

	resp.Stats["messages"] = len(resp.Messages)
	resp.Stats["enums"] = len(resp.Enums)
	resp.Stats["packages"] = len(resp.Packages)

	return resp
}

func extractProtoPackage(fullName string) string {
	// fullName is like "waE2E.Message.SubMessage" — package is the first component
	parts := strings.SplitN(fullName, ".", 2)
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

func cardinalityLabel(c protoreflect.Cardinality) string {
	switch c {
	case protoreflect.Optional:
		return "optional"
	case protoreflect.Required:
		return "required"
	case protoreflect.Repeated:
		return "repeated"
	default:
		return "optional"
	}
}

// ── handlers ─────────────────────────────────────────────────────────────────

// getProtoSchema returns the full protobuf schema of all registered WhatsApp proto types.
func getProtoSchema(e *core.RequestEvent) error {
	protoSchemaOnce.Do(func() {
		protoSchemaCache = buildProtoSchema()
	})
	return e.JSON(http.StatusOK, protoSchemaCache)
}

// getProtoMessage returns detail for a single message type by full name.
func getProtoMessage(e *core.RequestEvent) error {
	name := e.Request.URL.Query().Get("name")
	if name == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "name query param required"})
	}

	mt, err := protoregistry.GlobalTypes.FindMessageByName(protoreflect.FullName(name))
	if err != nil {
		return e.JSON(http.StatusNotFound, map[string]string{"error": "message type not found"})
	}

	md := mt.Descriptor()
	msg := protoMessageDesc{
		FullName: string(md.FullName()),
		Package:  extractProtoPackage(string(md.FullName())),
	}

	fields := md.Fields()
	for i := range fields.Len() {
		fd := fields.Get(i)
		f := protoFieldDesc{
			Number: int32(fd.Number()),
			Name:   string(fd.Name()),
			Type:   fd.Kind().String(),
			Label:  cardinalityLabel(fd.Cardinality()),
		}
		if fd.ContainingOneof() != nil {
			f.OneOf = string(fd.ContainingOneof().Name())
		}
		switch fd.Kind() {
		case protoreflect.MessageKind, protoreflect.GroupKind:
			if fd.Message() != nil {
				f.TypeRef = string(fd.Message().FullName())
			}
		case protoreflect.EnumKind:
			if fd.Enum() != nil {
				f.TypeRef = string(fd.Enum().FullName())
			}
		}
		msg.Fields = append(msg.Fields, f)
	}

	oneofs := md.Oneofs()
	for i := range oneofs.Len() {
		msg.OneOfs = append(msg.OneOfs, string(oneofs.Get(i).Name()))
	}

	nested := md.Messages()
	for i := range nested.Len() {
		msg.Nested = append(msg.Nested, string(nested.Get(i).FullName()))
	}

	enums := md.Enums()
	for i := range enums.Len() {
		msg.Enums = append(msg.Enums, string(enums.Get(i).FullName()))
	}

	return e.JSON(http.StatusOK, msg)
}
