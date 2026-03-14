package stages

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component/loki/process/stages/fix"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/loki/pkg/push"
)

var (
	ErrEmptyFixStageSource = errors.New("empty source")
	ErrEmptyFixStageDelimiter    = errors.New("empty delimiter")
	ErrEmptyFixStageVersion    = errors.New("empty fix version")
	ErrInvalidFixStageVersion    = errors.New("invalid fix version")
)

type FixConfig struct {
	Source *string `alloy:"source,attr,optional"`
	Delimiter    *string `alloy:"delimiter,attr,optional"`
	FixVersion *string `alloy:"fix_version,attr,optional"`
}

func validateFixConfig(c FixConfig) error {
	if c.Source != nil && *c.Source == "" {
		return ErrEmptyFixStageSource
	}
	if c.Delimiter != nil && *c.Delimiter == "" {
		return ErrEmptyFixStageDelimiter
	}
	if c.FixVersion != nil && *c.FixVersion == "" {
		return ErrEmptyFixStageVersion
	}
	if c.FixVersion != nil && !fix.IsVersionValid(*c.FixVersion) {
		return ErrInvalidFixStageVersion
	}
	return nil
}

type fixStage struct {
	config *FixConfig
	logger log.Logger
}

func newFixStage(logger log.Logger, config FixConfig) (Stage, error) {
	err := validateFixConfig(config)
	if err != nil {
		return nil, err
	}
	return &fixStage{
		config: &config,
		logger: log.With(logger, "component", "stage", "type", "fix"),
	}, nil
}

func (*fixStage) Cleanup() {
	// no-op
}

func (r *fixStage) Run(in chan Entry) chan Entry {
	return RunWith(in, func(e Entry) Entry {
		input := &e.Line
		delimiter := string('\x01')
		fix_version := "4.4"

		if r.config.Delimiter != nil {
			delimiter = *r.config.Delimiter
		}
		if r.config.FixVersion != nil {
			fix_version = *r.config.FixVersion
		}

		if r.config.Source != nil {
			if _, ok := e.Extracted[*r.config.Source]; !ok {
				if Debug {
					level.Debug(r.logger).Log("msg", "source does not exist in the set of extracted values", "source", *r.config.Source)
				}
				return e
			}

			value, err := getString(e.Extracted[*r.config.Source])
			if err != nil {
				if Debug {
					level.Debug(r.logger).Log("msg", "failed to convert source value to string", "source", *r.config.Source, "err", err, "type", reflect.TypeOf(e.Extracted[*r.config.Source]))
				}
				return e
			}

			input = &value
		}

		if input == nil {
			if Debug {
				level.Debug(r.logger).Log("msg", "cannot parse a nil entry")
			}
			return e
		}

		level.Info(r.logger).Log("msg", "INPUT VALUE", "input", fmt.Sprintf("%q", *input))
		if !strings.HasPrefix(*input, "8=FIX") {
			if Debug {
				level.Debug(r.logger).Log("msg", "failed to validate a fix message")
			}
			return e
		}

		var tagMapping map[string]string
		var msgTypeMapping map[string]string

		switch fix_version {
		case "1.1":
			tagMapping = fix.TagsMapping11
			msgTypeMapping = fix.MsgTypeMapping11
		case "4.0":
			tagMapping = fix.TagsMapping40
			msgTypeMapping = fix.MsgTypeMapping40
		case "4.1":
			tagMapping = fix.TagsMapping41
			msgTypeMapping = fix.MsgTypeMapping41
		case "4.2":
			tagMapping = fix.TagsMapping42
			msgTypeMapping = fix.MsgTypeMapping42
		case "4.3":
			tagMapping = fix.TagsMapping43
			msgTypeMapping = fix.MsgTypeMapping43
		case "4.4":
			tagMapping = fix.TagsMapping44
			msgTypeMapping = fix.MsgTypeMapping44
		case "5.0":
			tagMapping = fix.TagsMapping50
			msgTypeMapping = fix.MsgTypeMapping50
		default:
			level.Error(r.logger).Log("msg", fmt.Sprintf("couldn't find tag mapping for fix version %s", fix_version))
			return e
		}

		for tag := range strings.SplitSeq(*input, delimiter) {
			key, val, ok := strings.Cut(tag, "=")
			if !ok {
				continue
			}

			if key == "35" {
				if msgType, ok := msgTypeMapping[val]; ok {
					e.StructuredMetadata = append(e.StructuredMetadata, push.LabelAdapter{Name: "FixMsgType", Value: msgType})
				}
			}

			if tagName, ok := tagMapping[key]; ok {
				key = tagName
			}
			e.Extracted[key] = val
			e.StructuredMetadata = append(e.StructuredMetadata, push.LabelAdapter{Name: key, Value: val})
		}
		return e
	})
}
