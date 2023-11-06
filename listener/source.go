package listener

import (
	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
)

type PacketSource interface {
	Packets() chan gopacket.Packet
}

var _ PacketSource = (*gopacket.PacketSource)(nil)

func NewPacketSourceLive(device, filter string) (PacketSource, error) {
	handle, err := pcap.OpenLive(device, 65535, true, pcap.BlockForever)
	if err != nil {
		return nil, err
	}
	if err = handle.SetBPFFilter(filter); err != nil {
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
