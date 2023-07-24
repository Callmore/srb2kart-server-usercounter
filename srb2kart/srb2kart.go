package srb2kart

import (
	"bytes"
	"encoding/binary"
	"net"
	"time"
)

const (
	maxApplication  = 16
	maxServerName   = 32
	maxMirrorLength = 256
	maxFileNeeded   = 915
	maxWadPath      = 512
)

type packetType uint8

const (
	packetTypeAskInfo    packetType = 12 // PT_ASKINFO
	packetTypeServerInfo packetType = 13 // PT_SERVERINFO
	packetTypeClientInfo packetType = 14 // PT_PLAYERINFO
)

type packetHeader struct {
	Checksum   uint32
	Ack        uint8
	AckReturn  uint8
	PacketType packetType
	_          uint8
}

type packetAskInfo struct {
	Version uint8
	Time    uint32
}

type packetServerInfo struct {
	V255           uint8
	PacketVersion  uint8
	Application    [maxApplication]byte
	Version        uint8
	SubVersion     uint8
	NumberOfPlayer uint8
	MaxPlayer      uint8
	Gametype       uint8
	ModifiedGame   uint8
	CheatsEnabled  uint8
	KartVars       uint8 // Previously isdedicated, now appropriated for our own nefarious purposes
	FileNeededNum  uint8
	Time           uint32
	LevelTime      uint32
	ServerName     [maxServerName]byte
	MapName        [8]byte
	MapTitle       [33]byte
	MapMD5         [16]byte
	ActNum         uint8
	IsZone         uint8
	HTTPSource     [maxMirrorLength]byte // HTTP URL to download from, always defined for compatibility
	FileNeeded     [maxFileNeeded]byte   // is filled with writexxx (byteptr.h)
}

func generateChecksum(data []byte) int32 {
	c := uint32(0x1234567)
	for i, v := range data[4:] {
		c += uint32(v) * uint32(i+1)
	}
	return int32(c)
}

func writeChecksum(data []byte) []byte {
	output := make([]byte, len(data))
	copy(output[4:], data[4:])
	c := generateChecksum(data)
	binary.LittleEndian.PutUint32(output, uint32(c))
	return output
}

type ServerInfo struct {
	IP            string
	ServerNameRaw []byte
	Players       int
	MaxPlayers    int
}

func GetServerInfo(address string) (ServerInfo, error) {
	conn, err := net.Dial("udp", address)
	if err != nil {
		return ServerInfo{}, err
	}
	defer conn.Close()

	packet := struct {
		packetHeader
		packetAskInfo
	}{
		packetHeader: packetHeader{
			PacketType: packetTypeAskInfo,
		},
		packetAskInfo: packetAskInfo{},
	}
	buf := bytes.Buffer{}
	err = binary.Write(&buf, binary.LittleEndian, packet)
	if err != nil {
		return ServerInfo{}, err
	}

	_, err = conn.Write(writeChecksum(buf.Bytes()))
	if err != nil {
		return ServerInfo{}, err
	}

	serverInfoResponse := packetServerInfo{}

	for i := 0; i < 2; i++ {
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))

		buf := make([]byte, 2048)
		_, err := conn.Read(buf)
		if err != nil {
			return ServerInfo{}, err
		}

		reader := bytes.NewReader(buf)
		var header packetHeader
		binary.Read(reader, binary.LittleEndian, &header)
		switch header.PacketType {
		case packetTypeServerInfo:
			binary.Read(reader, binary.LittleEndian, &serverInfoResponse)
		case packetTypeClientInfo:
		default:
		}
	}

	return ServerInfo{
		IP:            address,
		ServerNameRaw: spliceAtNull(serverInfoResponse.ServerName[:]),
		Players:       int(serverInfoResponse.NumberOfPlayer),
		MaxPlayers:    int(serverInfoResponse.MaxPlayer),
	}, nil
}

func spliceAtNull(s []byte) []byte {
	for i, v := range s {
		if v == 0x00 {
			return s[:i]
		}
	}
	return s
}
