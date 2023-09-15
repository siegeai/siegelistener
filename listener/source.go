package listener

import (
	"fmt"
	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
)

type PacketSource interface {
	Packets() chan gopacket.Packet
}

var _ PacketSource = (*gopacket.PacketSource)(nil)

func NewPacketSourceLive(device string, port int) (PacketSource, error) {
	handle, err := pcap.OpenLive(device, 65535, true, pcap.BlockForever)
	if err != nil {
		return nil, err
	}
	if err = handle.SetBPFFilter(fmt.Sprintf("tcp and port %d", port)); err != nil {
		return nil, err
	}
	return gopacket.NewPacketSource(handle, handle.LinkType()), nil
}

//func NewPacketSourceFile(fileName string) (PacketSource, error) {
//	handle, err := pcap.OpenOffline(fileName)
//	if err != nil {
//		return nil, err
//	}
//	if err = handle.SetBPFFilter("tcp"); err != nil {
//		return nil, err
//	}
//	return gopacket.NewPacketSource(handle, handle.LinkType()), nil
//}
