package field

import (
	"fmt"
	"testing"

	"github.com/moov-io/iso8583/encoding"
	"github.com/moov-io/iso8583/padding"
	"github.com/moov-io/iso8583/prefix"
	"github.com/stretchr/testify/require"
)

var (
	compositeTestSpec = &Spec{
		Length:      6,
		Description: "Test Spec",
		Pref:        prefix.ASCII.Fixed,
		Pad:         padding.None,
		Subfields: map[string]Field{
			"1": NewString(&Spec{
				Length:      2,
				Description: "String Field",
				Enc:         encoding.ASCII,
				Pref:        prefix.ASCII.Fixed,
			}),
			"2": NewString(&Spec{
				Length:      2,
				Description: "String Field",
				Enc:         encoding.ASCII,
				Pref:        prefix.ASCII.Fixed,
			}),
			"3": NewNumeric(&Spec{
				Length:      2,
				Description: "Numeric Field",
				Enc:         encoding.ASCII,
				Pref:        prefix.ASCII.Fixed,
			}),
		},
	}
	compositeTestSpecWithIDLength = &Spec{
		Length:      30,
		Description: "Test Spec",
		Pref:        prefix.ASCII.LL,
		Tag: &TagSpec{
			Length: 2,
			Enc:    encoding.ASCII,
			Pad:    padding.Left('0'),
		},
		Subfields: map[string]Field{
			"1": NewString(&Spec{
				Length:      2,
				Description: "String Field",
				Enc:         encoding.ASCII,
				Pref:        prefix.ASCII.LL,
			}),
			"2": NewString(&Spec{
				Length:      2,
				Description: "String Field",
				Enc:         encoding.ASCII,
				Pref:        prefix.ASCII.LL,
			}),
			"3": NewNumeric(&Spec{
				Length:      2,
				Description: "Numeric Field",
				Enc:         encoding.ASCII,
				Pref:        prefix.ASCII.LL,
			}),
			"4": NewComposite(&Spec{
				Length:      6,
				Description: "Sub-Composite Field",
				Pref:        prefix.ASCII.LL,
				Tag: &TagSpec{
					Length: 2,
					Enc:    encoding.ASCII,
					Pad:    padding.Left('0'),
				},
				Subfields: map[string]Field{
					"1": NewString(&Spec{
						Length:      2,
						Description: "String Field",
						Enc:         encoding.ASCII,
						Pref:        prefix.ASCII.LL,
					}),
				},
			}),
		},
	}

	tlvTestSpec = &Spec{
		Length:      999,
		Description: "ICC Data – EMV Having Multiple Tags",
		Pref:        prefix.ASCII.LLL,
		Tag: &TagSpec{
			Enc: encoding.BerTLVTag,
		},
		Subfields: map[string]Field{
			"9A": NewString(&Spec{
				Description: "Transaction Date",
				Enc:         encoding.ASCIIToHex,
				Pref:        prefix.BerTLV,
			}),
			"9F02": NewString(&Spec{
				Description: "Amount, Authorized (Numeric)",
				Enc:         encoding.ASCIIToHex,
				Pref:        prefix.BerTLV,
			}),
		},
	}
)

type CompsiteTestData struct {
	F1 *String
	F2 *String
	F3 *Numeric
	F4 *SubCompositeData
}

type SubCompositeData struct {
	F1 *String
}

type TLVTestData struct {
	F9A   *String
	F9F02 *String
}

func TestComposite_SetData(t *testing.T) {
	t.Run("SetData returns an error on provision of primitive type data", func(t *testing.T) {
		composite := NewComposite(compositeTestSpec)
		err := composite.SetData("primitive str")
		require.EqualError(t, err, "failed to set data as struct is expected, got: string")
	})
}

func TestTLVPacking(t *testing.T) {
	t.Run("Pack correctly serializes data to bytes", func(t *testing.T) {
		data := &TLVTestData{
			F9A:   NewStringValue("210720"),
			F9F02: NewStringValue("000000000501"),
		}

		composite := NewComposite(tlvTestSpec)
		err := composite.SetData(data)
		require.NoError(t, err)

		packed, err := composite.Pack()
		require.NoError(t, err)

		// TLV Length: 0x30, 0x31, 0x34 (014)
		//
		// Tag: 0x9a (9A)
		// Length: 0x03 (3 bytes)
		// Value: 0x21, 0x07, 0x20 (210720)
		//
		// Tag: 0x9f, 0x02 (9F02)
		// Length: 0x06 (6 bytes)
		// Value: 0x00, 0x00, 0x00, 0x00, 0x05, 0x01 (000000000501)
		require.Equal(t, []byte{0x30, 0x31, 0x34, 0x9a, 0x3, 0x21, 0x7, 0x20, 0x9f, 0x2, 0x6, 0x0, 0x0, 0x0, 0x0, 0x5, 0x1}, packed)
	})

	t.Run("Unpack correctly deserialises bytes to the data struct", func(t *testing.T) {
		data := &TLVTestData{}

		composite := NewComposite(tlvTestSpec)
		err := composite.SetData(data)
		require.NoError(t, err)

		read, err := composite.Unpack([]byte{0x30, 0x31, 0x34, 0x9a, 0x3, 0x21, 0x7, 0x20, 0x9f, 0x2, 0x6, 0x0, 0x0, 0x0, 0x0, 0x5, 0x1})
		require.NoError(t, err)
		require.Equal(t, 17, read)

		require.Equal(t, "210720", data.F9A.Value)
		require.Equal(t, "000000000501", data.F9F02.Value)
	})
}

func TestCompositePacking(t *testing.T) {
	t.Run("Pack returns an error on mismatch of subfield types", func(t *testing.T) {
		type TestDataIncorrectType struct {
			F1 *Numeric
		}
		composite := NewComposite(compositeTestSpec)
		err := composite.SetData(&TestDataIncorrectType{
			F1: NewNumericValue(1),
		})
		require.NoError(t, err)

		_, err = composite.Pack()
		require.EqualError(t, err, "failed to set data for field 1: data does not match required *String type")
	})

	t.Run("Pack returns error on failure of subfield packing", func(t *testing.T) {
		data := &CompsiteTestData{
			// This subfield will return an error on F1.Pack() as its length
			// exceeds the max length defined in the spec.
			F1: NewStringValue("ABCD"),
			F2: NewStringValue("CD"),
			F3: NewNumericValue(12),
		}

		composite := NewComposite(compositeTestSpec)
		err := composite.SetData(data)
		require.NoError(t, err)

		_, err = composite.Pack()
		require.EqualError(t, err, "failed to pack subfield 1: failed to encode length: field length: 4 should be fixed: 2")
	})

	t.Run("Pack returns error on failure to encode length", func(t *testing.T) {
		invalidSpec := &Spec{
			// Base field length < summation of lengths of subfields
			// This will throw an error when encoding the field's length.
			Length: 4,
			Pref:   prefix.ASCII.Fixed,
			Subfields: map[string]Field{
				"1": NewString(&Spec{
					Length: 2,
					Enc:    encoding.ASCII,
					Pref:   prefix.ASCII.Fixed,
				}),
				"2": NewString(&Spec{
					Length: 2,
					Enc:    encoding.ASCII,
					Pref:   prefix.ASCII.Fixed,
				}),
				"3": NewNumeric(&Spec{
					Length: 2,
					Enc:    encoding.ASCII,
					Pref:   prefix.ASCII.Fixed,
				}),
			},
		}
		data := &CompsiteTestData{
			F1: NewStringValue("AB"),
			F2: NewStringValue("CD"),
			F3: NewNumericValue(12),
		}

		composite := NewComposite(invalidSpec)
		err := composite.SetData(data)
		require.NoError(t, err)

		_, err = composite.Pack()
		require.EqualError(t, err, "failed to encode length: field length: 6 should be fixed: 4")
	})

	t.Run("Pack correctly serializes data to bytes", func(t *testing.T) {
		data := &CompsiteTestData{
			F1: NewStringValue("AB"),
			F2: NewStringValue("CD"),
			F3: NewNumericValue(12),
		}

		composite := NewComposite(compositeTestSpec)
		err := composite.SetData(data)
		require.NoError(t, err)

		packed, err := composite.Pack()
		require.NoError(t, err)

		require.NoError(t, err)
		require.Equal(t, "ABCD12", string(packed))
	})

	t.Run("Unpack returns an error on mismatch of subfield types", func(t *testing.T) {
		type TestDataIncorrectType struct {
			F1 *Numeric
		}
		composite := NewComposite(compositeTestSpec)
		err := composite.SetData(&TestDataIncorrectType{})
		require.NoError(t, err)

		read, err := composite.Unpack([]byte("ABCD12"))
		require.Equal(t, 0, read)
		require.Error(t, err)
		require.EqualError(t, err, "failed to set data for field 1: data does not match required *String type")
	})

	t.Run("Unpack returns an error on failure of subfield to unpack bytes", func(t *testing.T) {
		data := &CompsiteTestData{}

		composite := NewComposite(compositeTestSpec)
		err := composite.SetData(data)
		require.NoError(t, err)

		// Last two characters must be an integer type. F3 fails to unpack.
		read, err := composite.Unpack([]byte("ABCDEF"))
		require.Equal(t, 0, read)
		require.Error(t, err)
		require.EqualError(t, err, "failed to unpack subfield 3: failed to convert into number: strconv.Atoi: parsing \"EF\": invalid syntax")
	})

	t.Run("Unpack returns an error on length of data exceeding max length", func(t *testing.T) {
		spec := &Spec{
			Length: 4,
			Pref:   prefix.ASCII.L,
			Subfields: map[string]Field{
				"1": NewString(&Spec{
					Length: 2,
					Enc:    encoding.ASCII,
					Pref:   prefix.ASCII.Fixed,
				}),
				"2": NewString(&Spec{
					Length: 2,
					Enc:    encoding.ASCII,
					Pref:   prefix.ASCII.Fixed,
				}),
				"3": NewNumeric(&Spec{
					Length: 2,
					Enc:    encoding.ASCII,
					Pref:   prefix.ASCII.Fixed,
				}),
			},
		}
		data := &CompsiteTestData{}

		composite := NewComposite(spec)
		err := composite.SetData(data)
		require.NoError(t, err)

		// Length of denoted by prefix is too long, causing failure to decode length.
		read, err := composite.Unpack([]byte("7ABCD123"))
		require.Equal(t, 0, read)
		require.Error(t, err)
		require.EqualError(t, err, "failed to decode length: data length: 7 is larger than maximum 4")
	})

	t.Run("Unpack returns an error on offset not matching data length", func(t *testing.T) {
		invalidSpec := &Spec{
			// Base field length < summation of lengths of subfields
			// This will throw an error when encoding the field's length.
			Length: 4,
			Pref:   prefix.ASCII.Fixed,
			Subfields: map[string]Field{
				"1": NewString(&Spec{
					Length: 2,
					Enc:    encoding.ASCII,
					Pref:   prefix.ASCII.Fixed,
				}),
				"2": NewString(&Spec{
					Length: 2,
					Enc:    encoding.ASCII,
					Pref:   prefix.ASCII.Fixed,
				}),
				"3": NewNumeric(&Spec{
					Length: 3,
					Enc:    encoding.ASCII,
					Pref:   prefix.ASCII.Fixed,
				}),
			},
		}
		data := &CompsiteTestData{}

		composite := NewComposite(invalidSpec)
		err := composite.SetData(data)
		require.NoError(t, err)

		// Length of input too long, causing failure to decode length.
		read, err := composite.Unpack([]byte("ABCD123"))
		require.Equal(t, 0, read)
		require.Error(t, err)
		require.EqualError(t, err, "data length: 4 does not match aggregate data read from decoded subfields: 7")
	})

	t.Run("Unpack correctly deserialises bytes to the data struct", func(t *testing.T) {
		data := &CompsiteTestData{}

		composite := NewComposite(compositeTestSpec)
		err := composite.SetData(data)
		require.NoError(t, err)

		read, err := composite.Unpack([]byte("ABCD12"))
		require.Equal(t, compositeTestSpec.Length, read)
		require.NoError(t, err)

		require.Equal(t, "AB", data.F1.Value)
		require.Equal(t, "CD", data.F2.Value)
		require.Equal(t, 12, data.F3.Value)
		require.Nil(t, data.F4)
	})
}

func TestCompositePackingWithID(t *testing.T) {
	t.Run("Pack returns error on failure to encode length", func(t *testing.T) {
		// Base field length < summation of (lengths of subfields + IDs).
		// This will throw an error when encoding the field's length.
		invalidSpec := &Spec{
			Length: 6,
			Pref:   prefix.ASCII.Fixed,
			Tag: &TagSpec{
				Length: 2,
				Enc:    encoding.ASCII,
				Pad:    padding.Left('0'),
			},
			Subfields: map[string]Field{
				"1": NewString(&Spec{
					Length: 2,
					Enc:    encoding.ASCII,
					Pref:   prefix.ASCII.Fixed,
				}),
				"2": NewString(&Spec{
					Length: 2,
					Enc:    encoding.ASCII,
					Pref:   prefix.ASCII.Fixed,
				}),
				"3": NewNumeric(&Spec{
					Length: 2,
					Enc:    encoding.ASCII,
					Pref:   prefix.ASCII.Fixed,
				}),
			},
		}
		data := &CompsiteTestData{
			F1: NewStringValue("AB"),
			F2: NewStringValue("CD"),
			F3: NewNumericValue(12),
		}

		composite := NewComposite(invalidSpec)
		err := composite.SetData(data)
		require.NoError(t, err)

		b, err := composite.Pack()
		require.Nil(t, b)
		require.Error(t, err)
		require.EqualError(t, err, "failed to encode length: field length: 12 should be fixed: 6")
	})

	t.Run("Pack correctly serializes fully populated data to bytes", func(t *testing.T) {
		data := &CompsiteTestData{
			F1: NewStringValue("AB"),
			F2: NewStringValue("CD"),
			F3: NewNumericValue(12),
			F4: &SubCompositeData{
				F1: NewStringValue("YZ"),
			},
		}

		composite := NewComposite(compositeTestSpecWithIDLength)
		err := composite.SetData(data)
		require.NoError(t, err)

		packed, err := composite.Pack()
		require.NoError(t, err)

		require.Equal(t, "280102AB0202CD03021204060102YZ", string(packed))
	})

	t.Run("Pack correctly serializes partially populated data to bytes", func(t *testing.T) {
		data := &CompsiteTestData{
			F1: NewStringValue("AB"),
			F3: NewNumericValue(12),
		}

		composite := NewComposite(compositeTestSpecWithIDLength)
		err := composite.SetData(data)
		require.NoError(t, err)

		packed, err := composite.Pack()
		require.NoError(t, err)

		require.NoError(t, err)
		require.Equal(t, "120102AB030212", string(packed))
	})

	t.Run("Unpack returns an error on failure of subfield to unpack bytes", func(t *testing.T) {
		data := &CompsiteTestData{}

		composite := NewComposite(compositeTestSpecWithIDLength)
		err := composite.SetData(data)
		require.NoError(t, err)

		// F3 fails to unpack - it requires len to be defined instead of AB.
		read, err := composite.Unpack([]byte("180102AB0202CD03AB12"))
		require.Equal(t, 0, read)
		require.Error(t, err)
		require.EqualError(t, err, "failed to unpack subfield 3: failed to decode length: strconv.Atoi: parsing \"AB\": invalid syntax")
	})

	t.Run("Unpack returns an error on data having subfield ID not in spec", func(t *testing.T) {
		data := &CompsiteTestData{}

		composite := NewComposite(compositeTestSpecWithIDLength)
		err := composite.SetData(data)
		require.NoError(t, err)

		// Index 2-3 should have '01' rather than '11'.
		read, err := composite.Unpack([]byte("181102AB0202CD030212"))
		require.Equal(t, 0, read)
		require.EqualError(t, err, "failed to unpack subfield 11: field not defined in Spec")
	})

	t.Run("Unpack returns an error on if subfield not defined in spec", func(t *testing.T) {
		data := &CompsiteTestData{}

		composite := NewComposite(compositeTestSpecWithIDLength)
		err := composite.SetData(data)
		require.NoError(t, err)

		// Index 0, 1 should have '01' rather than 'ID'.
		read, err := composite.Unpack([]byte("18ID02AB0202CD030212"))
		require.Equal(t, 0, read)
		require.EqualError(t, err, "failed to unpack subfield ID: field not defined in Spec")
	})

	t.Run("Unpack correctly deserialises out of order composite subfields to the data struct", func(t *testing.T) {
		data := &CompsiteTestData{}

		composite := NewComposite(compositeTestSpecWithIDLength)
		err := composite.SetData(data)
		require.NoError(t, err)

		read, err := composite.Unpack([]byte("280202CD0302120102AB04060102YZ"))

		require.NoError(t, err)
		require.Equal(t, 30, read)

		require.Equal(t, "AB", data.F1.Value)
		require.Equal(t, "CD", data.F2.Value)
		require.Equal(t, 12, data.F3.Value)
		require.Equal(t, "YZ", data.F4.F1.Value)
	})

	t.Run("Unpack correctly deserialises partial subfields to the data struct", func(t *testing.T) {
		data := &CompsiteTestData{}

		composite := NewComposite(compositeTestSpecWithIDLength)
		err := composite.SetData(data)
		require.NoError(t, err)

		read, err := composite.Unpack([]byte("120302120102AB"))

		require.NoError(t, err)
		require.Equal(t, 14, read)

		require.Equal(t, "AB", data.F1.Value)
		require.Nil(t, data.F2)
		require.Equal(t, 12, data.F3.Value)
	})

	t.Run("Unpack correctly ignores excess bytes in excess of the length described by the prefix", func(t *testing.T) {
		data := &CompsiteTestData{}

		composite := NewComposite(compositeTestSpecWithIDLength)
		err := composite.SetData(data)
		require.NoError(t, err)

		// "04060102YZ" falls outside of the bounds of the 18 byte limit imposed
		// by the prefix. Therefore, F4 must be nil.
		read, err := composite.Unpack([]byte("180202CD0302120102AB04060102YZ"))

		require.NoError(t, err)
		require.Equal(t, 20, read)

		require.Equal(t, "AB", data.F1.Value)
		require.Equal(t, "CD", data.F2.Value)
		require.Equal(t, 12, data.F3.Value)
		require.Nil(t, data.F4)
	})
}

func TestCompositeHandlesValidSpecs(t *testing.T) {
	tests := []struct {
		desc string
		spec *Spec
	}{
		{
			desc: "accepts nil Enc value",
			spec: &Spec{
				Length:    6,
				Pref:      prefix.ASCII.Fixed,
				Subfields: map[string]Field{},
			},
		},
		{
			desc: "accepts nil Pad value",
			spec: &Spec{
				Length:    6,
				Pref:      prefix.ASCII.Fixed,
				Subfields: map[string]Field{},
			},
		},
		{
			desc: "accepts None Pad value",
			spec: &Spec{
				Length:    6,
				Pref:      prefix.ASCII.Fixed,
				Pad:       padding.None,
				Subfields: map[string]Field{},
			},
		},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("NewComposite() %v", tc.desc), func(t *testing.T) {
			f := NewComposite(tc.spec)
			require.Equal(t, tc.spec, f.Spec())
		})
		t.Run(fmt.Sprintf("Composite.SetSpec() %v", tc.desc), func(t *testing.T) {
			f := &Composite{}
			f.SetSpec(tc.spec)
			require.Equal(t, tc.spec, f.Spec())
		})
	}
}

func TestCompositePanicsOnSpecValidationFailures(t *testing.T) {
	tests := []struct {
		desc string
		err  string
		spec *Spec
	}{
		{
			desc: "panics on non-None / non-nil Pad value being defined in spec",
			err:  "Composite spec only supports nil or None spec padding values",
			spec: &Spec{
				Length:    6,
				Pref:      prefix.ASCII.Fixed,
				Pad:       padding.Left('0'),
				Subfields: map[string]Field{},
			},
		},
		{
			desc: "panics on non-nil Enc value being defined in spec",
			err:  "Composite spec only supports a nil Enc value",
			spec: &Spec{
				Length:    6,
				Enc:       encoding.ASCII,
				Pref:      prefix.ASCII.Fixed,
				Subfields: map[string]Field{},
			},
		},
		{
			desc: "panics on nil Enc value being defined in spec if IDLength > 0",
			err:  "Composite spec requires a Tag.Enc to be defined if Tag.Length > 0",
			spec: &Spec{
				Length:    6,
				Pref:      prefix.ASCII.Fixed,
				Subfields: map[string]Field{},
				Tag: &TagSpec{
					Length: 2,
					Pad:    padding.Left('0'),
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("NewComposite() %v", tc.desc), func(t *testing.T) {
			require.PanicsWithError(t, tc.err, func() {
				NewComposite(tc.spec)
			})
		})
		t.Run(fmt.Sprintf("Composite.SetSpec() %v", tc.desc), func(t *testing.T) {
			require.PanicsWithError(t, tc.err, func() {
				(&Composite{}).SetSpec(tc.spec)
			})
		})
	}
}
