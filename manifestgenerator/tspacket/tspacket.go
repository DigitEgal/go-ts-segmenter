package tspacket

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

const (
	// TsDefaultPacketSize Default TS packet size
	TsDefaultPacketSize = 188

	// TsStartByte Start byte for TS pakcets
	TsStartByte = 0x47
)

// transportPacketData TS packet info
type transportPacketData struct {
	valid                      bool
	SyncByte                   uint8
	TransportErrorIndicator    bool
	PayloadUnitStartIndicator  bool
	TransportPriority          bool
	PID                        uint16
	TransportScramblingControl uint8
	AdaptationFieldControl     uint8
	ContinuityCounter          uint8
	AdaptationField            transportPacketAdaptationFieldData
}

// Reset transportPacketData
func (t *transportPacketData) Reset() {
	t.valid = false
	t.SyncByte = 0
	t.TransportErrorIndicator = false
	t.PayloadUnitStartIndicator = false
	t.TransportErrorIndicator = false
	t.PID = 0
	t.TransportScramblingControl = 0
	t.AdaptationFieldControl = 0
	t.ContinuityCounter = 0
	t.AdaptationField.AdaptationFieldLength = 0
	t.AdaptationField.DiscontinuityIndicator = false
	t.AdaptationField.RandomAccessIndicator = false
	t.AdaptationField.ElementaryStreamPriorityIndicator = false
	t.AdaptationField.PCRFlag = false
	t.AdaptationField.OPCRFlag = false
	t.AdaptationField.SplicingPointFlag = false
	t.AdaptationField.TransportPrivateDataFlag = false
	t.AdaptationField.AdaptationFieldExtensionFlag = false
	t.AdaptationField.PCRData.valid = false
	t.AdaptationField.PCRData.ProgramClockReferenceBase = 0
	t.AdaptationField.PCRData.ProgramClockReferenceExtension = 0
	t.AdaptationField.PCRData.reserved = 0
	t.AdaptationField.PCRData.PCRs = 0
}

// transportPacketAdaptationFieldData TS adaptation field packet info
type transportPacketAdaptationFieldData struct {
	AdaptationFieldLength             uint8
	DiscontinuityIndicator            bool
	RandomAccessIndicator             bool
	ElementaryStreamPriorityIndicator bool
	PCRFlag                           bool
	OPCRFlag                          bool
	SplicingPointFlag                 bool
	TransportPrivateDataFlag          bool
	AdaptationFieldExtensionFlag      bool
	PCRData                           transportPacketAdaptationPCRFieldData
}

// transportPacketAdaptationPCRFieldData TS PCR field packet info
type transportPacketAdaptationPCRFieldData struct {
	ProgramClockReferenceBase      uint64
	reserved                       uint8
	ProgramClockReferenceExtension uint16
	PCRs                           float64
	valid                          bool
}

// TsPacket Transport stream packet
type TsPacket struct {
	buf             []byte
	lastIndex       int
	transportPacket transportPacketData
}

// New Creates a TsPacket instance
func New(packetSize int) TsPacket {
	p := TsPacket{make([]byte, packetSize), 0, *new(transportPacketData)}

	return p
}

// Reset packet
func (p *TsPacket) Reset() {
	p.lastIndex = 0
	p.transportPacket.Reset()
}

// AddData Adds bytes to the packet
func (p *TsPacket) AddData(buf []byte) {

	p.lastIndex = p.lastIndex + copy(p.buf[p.lastIndex:], buf[:])
}

// IsComplete Adds bytes to the packet
func (p *TsPacket) IsComplete() bool {
	if p.lastIndex == TsDefaultPacketSize && p.buf[0] == TsStartByte {
		return true
	}
	return false
}

// Parse Parse the packet
func (p *TsPacket) Parse() bool {
	if !p.IsComplete() {
		return false
	}

	var transportPacket struct {
		SyncByte                      uint8
		ErrorIndicatorPayloadUnitPid  uint16
		ScrambledAdapFieldContCounter uint8
	}

	r := bytes.NewReader(p.buf)
	err := binary.Read(r, binary.BigEndian, &transportPacket)
	if err != nil {
		return false
	}
	p.transportPacket.Reset()

	p.transportPacket.SyncByte = transportPacket.SyncByte
	if transportPacket.ErrorIndicatorPayloadUnitPid&0x8000 > 0 {
		p.transportPacket.TransportErrorIndicator = true
	}
	if transportPacket.ErrorIndicatorPayloadUnitPid&0x4000 > 0 {
		p.transportPacket.PayloadUnitStartIndicator = true
	}
	if transportPacket.ErrorIndicatorPayloadUnitPid&0x2000 > 0 {
		p.transportPacket.TransportPriority = true
	}
	p.transportPacket.PID = transportPacket.ErrorIndicatorPayloadUnitPid & 0x1FFF

	p.transportPacket.TransportScramblingControl = (transportPacket.ScrambledAdapFieldContCounter & 0xC0) >> 6
	p.transportPacket.AdaptationFieldControl = (transportPacket.ScrambledAdapFieldContCounter & 0x30) >> 4
	p.transportPacket.ContinuityCounter = transportPacket.ScrambledAdapFieldContCounter & 0x0F

	if p.transportPacket.AdaptationFieldControl == 2 || p.transportPacket.AdaptationFieldControl == 3 {
		var adaptationFieldLength uint8
		err := binary.Read(r, binary.BigEndian, &adaptationFieldLength)
		if err != nil {
			return false
		}

		if adaptationFieldLength > 0 {
			var adaptationFieldFlags uint8
			err := binary.Read(r, binary.BigEndian, &adaptationFieldFlags)
			if err != nil {
				return false
			}
			if (adaptationFieldFlags & 0x80) > 0 {
				p.transportPacket.AdaptationField.DiscontinuityIndicator = true
			}
			if (adaptationFieldFlags & 0x40) > 0 {
				p.transportPacket.AdaptationField.RandomAccessIndicator = true
			}
			if (adaptationFieldFlags & 0x20) > 0 {
				p.transportPacket.AdaptationField.ElementaryStreamPriorityIndicator = true
			}
			if (adaptationFieldFlags & 0x10) > 0 {
				p.transportPacket.AdaptationField.PCRFlag = true
			}
			if (adaptationFieldFlags & 0x08) > 0 {
				p.transportPacket.AdaptationField.OPCRFlag = true
			}
			if (adaptationFieldFlags & 0x04) > 0 {
				p.transportPacket.AdaptationField.SplicingPointFlag = true
			}
			if (adaptationFieldFlags & 0x02) > 0 {
				p.transportPacket.AdaptationField.TransportPrivateDataFlag = true
			}
			if (adaptationFieldFlags & 0x01) > 0 {
				p.transportPacket.AdaptationField.AdaptationFieldExtensionFlag = true
			}

			if p.transportPacket.AdaptationField.PCRFlag == true {
				var pcrDataFirst32b uint32
				err := binary.Read(r, binary.BigEndian, &pcrDataFirst32b)
				if err != nil {
					return false
				}

				var pcrDataLast16b uint16
				err = binary.Read(r, binary.BigEndian, &pcrDataLast16b)
				if err != nil {
					return false
				}
				p.transportPacket.AdaptationField.PCRData.ProgramClockReferenceExtension = uint16(pcrDataLast16b & 0x1FF)
				p.transportPacket.AdaptationField.PCRData.reserved = uint8((pcrDataLast16b >> 9) & 0x3F)

				p.transportPacket.AdaptationField.PCRData.ProgramClockReferenceBase = uint64(pcrDataFirst32b)*2 + uint64((pcrDataLast16b>>15)&0x1)

				p.transportPacket.AdaptationField.PCRData.PCRs = calculatePCRS(p.transportPacket.AdaptationField.PCRData.ProgramClockReferenceBase, p.transportPacket.AdaptationField.PCRData.ProgramClockReferenceExtension)

				p.transportPacket.AdaptationField.PCRData.valid = true
			}
		}
	}

	p.transportPacket.valid = true

	return true
}

func calculatePCRS(pcrBase uint64, pcrExtension uint16) (PCRs float64) {
	PCRs = -1

	if pcrExtension > 0 {
		PCRs = float64(pcrBase*300.0+uint64(pcrExtension)) / (27.0 * 1000000.0)
	} else {
		PCRs = float64(pcrBase) / 90000.0
	}

	return
}

// GetPCRS Get PCT in seconds
func (p *TsPacket) GetPCRS() (PCRs float64) {
	PCRs = -1
	if !p.transportPacket.valid || !p.transportPacket.AdaptationField.PCRData.valid {
		return
	}

	PCRs = p.transportPacket.AdaptationField.PCRData.PCRs

	return
}

// GetPID Adds bytes to the packet
func (p *TsPacket) GetPID() (pID int) {
	pID = -1
	if !p.transportPacket.valid {
		return
	}

	pID = int(p.transportPacket.PID)

	return
}

// ToString retuns packet data in string form
func (p *TsPacket) ToString() string {
	ret := ""
	if !p.transportPacket.valid {
		return ret
	}

	ret = fmt.Sprintf("%+v", p.transportPacket)
	return ret
}

// IsRandomAccess Return true if this is a random access point
func (p *TsPacket) IsRandomAccess(pID int) (isIDR bool) {
	isIDR = false
	if !p.transportPacket.valid {
		return
	}

	if p.transportPacket.PID == uint16(pID) {
		if p.transportPacket.AdaptationFieldControl == 2 || p.transportPacket.AdaptationFieldControl == 3 {
			if p.transportPacket.AdaptationField.RandomAccessIndicator == true {
				isIDR = true
			}
		}
	}

	return
}
