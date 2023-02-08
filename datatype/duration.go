package datatype

import (
	"time"

	"github.com/go-msvc/errors"
)

type Duration time.Duration

func (d Duration) Milliseconds() int64 {
	return int64(d.Duration().Seconds()) * 1000
} // Duration.Milliseconds()

func (d Duration) String() string {
	return d.Duration().String()
} // Duration.String()

func (d Duration) Value() (any, error) {
	return d.String(), nil
} // Duration.Value()

func (d *Duration) Scan(value interface{}) error {

	if value == nil {
		*d = Duration(0)
		return nil
	} // if value nil

	var byteValue []byte

	switch v := value.(type) {
	case []byte:
		byteValue = v
	case string:
		byteValue = []byte(v)
	default:
		return errors.Errorf("Duration cannot be initialised from %T %v",
			v, v)
	} // switch

	if err := d.UnmarshalText(byteValue); err != nil {
		return errors.Wrapf(err,
			"Failed to unmarshal %s", byteValue)
	} // if failed to unmarshal

	return nil

} // Duration.Scan ()

func (d Duration) MarshalText() ([]byte, error) {
	return []byte(d.String()), nil
} // Duration.MarshalJSON()

func (d *Duration) UnmarshalText(b []byte) error {

	if dur, err := time.ParseDuration(string(b)); err != nil {
		return errors.Wrapf(err,
			"Failed to parse duration [%s]",
			b)
	} else {
		*d = Duration(dur)
		return nil
	}

} // Duration.UnmarshalText()

func (d Duration) Duration() time.Duration {

	return time.Duration(d)

} // Duration.Duration()
