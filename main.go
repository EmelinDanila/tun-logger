package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"golang.zx2c4.com/wintun"
)

const (
	ifName      = "tunlogger"
	ifDesc      = "Test TUN interface for logging"
	logFileName = "packets.log"
	mtu         = 1500
)

func main() {
	// Создание лог-файла
	logFile, err := os.OpenFile(logFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		log.Fatalf("ошибка создания лог-файла: %v", err)
	}
	defer logFile.Close()
	logger := log.New(logFile, "", log.LstdFlags)

	// Создание интерфейса через wintun
	tun, err := wintun.CreateAdapter(ifName, "Wintun", nil)
	if err != nil {
		log.Fatalf("не удалось создать интерфейс: %v", err)
	}
	defer tun.Close()

	session, err := tun.StartSession(0x200000) // 2 MB
	if err != nil {
		log.Fatalf("не удалось начать сессию: %v", err)
	}
	defer session.End()

	log.Println("Интерфейс создан. Ожидание пакетов...")

	packetSource := make(chan []byte, 100) // Буферизированный канал

	// Чтение пакетов в отдельной горутине
	go func() {
		defer close(packetSource) // Закрываем канал при завершении горутины
		for {
			packet, err := session.ReceivePacket()
			if err != nil {
				if err.Error() == "No more data is available." {
					continue // Пропускаем, если данных нет
				}
				log.Printf("ошибка чтения пакета: %v", err)
				return // Выходим при фатальной ошибке
			}
			packetSource <- packet
			session.ReleaseReceivePacket(packet)
		}
	}()

	for pkt := range packetSource {
		parser := gopacket.NewPacket(pkt, layers.LayerTypeIPv4, gopacket.Default)

		ipLayer := parser.Layer(layers.LayerTypeIPv4)
		if ipLayer == nil {
			logger.Println("Не удалось разобрать IP-слой")
			continue
		}
		ip, _ := ipLayer.(*layers.IPv4)

		// Инициализируем строки
		srcIP := ip.SrcIP.String()
		dstIP := ip.DstIP.String()
		proto := ip.Protocol.String()

		info := fmt.Sprintf("[%s] %s -> %s [%s]", time.Now().Format("15:04:05"), srcIP, dstIP, proto)

		if tcpLayer := parser.Layer(layers.LayerTypeTCP); tcpLayer != nil {
			tcp := tcpLayer.(*layers.TCP)
			info += fmt.Sprintf(" TCP ports %d -> %d", tcp.SrcPort, tcp.DstPort)
		} else if udpLayer := parser.Layer(layers.LayerTypeUDP); udpLayer != nil {
			udp := udpLayer.(*layers.UDP)
			info += fmt.Sprintf(" UDP ports %d -> %d", udp.SrcPort, udp.DstPort)
		}

		logger.Println(info)
		logger.Println("Payload:")
		if appLayer := parser.ApplicationLayer(); appLayer != nil {
			logger.Println(hex.Dump(appLayer.Payload()))
		} else {
			logger.Println("Нет данных прикладного слоя")
		}
	}
}
