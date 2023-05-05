package main

import (
	"log"
	"machine"
	"time"
	"tinygo.org/x/bluetooth"
	"tinygo.org/x/drivers/bme280"
)

const ON bool = false
const OFF bool = true

var adapter = bluetooth.DefaultAdapter

var (
	serviceUUID = bluetooth.NewUUID([16]byte{0xa0, 0xb4, 0x00, 0x01, 0x92, 0x6d, 0x4d, 0x61, 0x98, 0xdf, 0x8c, 0x5c, 0x62, 0xee, 0x53, 0xb3})
	rxUUID      = bluetooth.NewUUID([16]byte{0xa0, 0xb4, 0x00, 0x02, 0x92, 0x6d, 0x4d, 0x61, 0x98, 0xdf, 0x8c, 0x5c, 0x62, 0xee, 0x53, 0xb3})
	txUUID      = bluetooth.NewUUID([16]byte{0xa0, 0xb4, 0x00, 0x03, 0x92, 0x6d, 0x4d, 0x61, 0x98, 0xdf, 0x8c, 0x5c, 0x62, 0xee, 0x53, 0xb3})
)

func getSensor(ch1 chan<- []byte) error {

	txData := make([]byte, 13)

	// センサ設定
	machine.I2C1.Configure(machine.I2CConfig{
		SCL: machine.SCL0_PIN,
		SDA: machine.SDA0_PIN,
	})
	sensor := bme280.New(machine.I2C1)
	sensor.Configure()

	// スイッチ設定
	sw := machine.D6
	sw.Configure(machine.PinConfig{Mode: machine.PinInputPullup})

	connected := sensor.Connected()
	if !connected {
		for {
			println("Error: BME280 not detected")
		}
	}

	for {
		// 温度情報取得
		temp, _ := sensor.ReadTemperature()
		hum, _ := sensor.ReadHumidity()
		pre, _ := sensor.ReadPressure()

		// データの分割
		txData[0] = byte(uint32(temp) & uint32(0xff))
		txData[1] = byte(uint32(temp) >> 8 & uint32(0xff))
		txData[2] = byte(uint32(temp) >> 16 & uint32(0xff))
		txData[3] = byte(uint32(temp) >> 24 & uint32(0xff))

		txData[4] = byte(uint32(hum) & uint32(0xff))
		txData[5] = byte(uint32(hum) >> 8 & uint32(0xff))
		txData[6] = byte(uint32(hum) >> 16 & uint32(0xff))
		txData[7] = byte(uint32(hum) >> 24 & uint32(0xff))

		txData[8] = byte(uint32(pre) & uint32(0xff))
		txData[9] = byte(uint32(pre) >> 8 & uint32(0xff))
		txData[10] = byte(uint32(pre) >> 16 & uint32(0xff))
		txData[11] = byte(uint32(pre) >> 24 & uint32(0xff))

		if sw.Get() == OFF {
			txData[12] = 0x00
		} else {
			txData[12] = 0x01

		}
		// センサなどの情報送信
		ch1 <- txData
		time.Sleep(100 * time.Millisecond)
	}

	return nil
}

var v uint8 = 0

func main() {

	err := run()
	if err != nil {
		log.Fatal(err)
	}

}

func run() error {

	// 出力ピンの設定
	led3 := machine.D9
	led2 := machine.D8
	led1 := machine.D7
	bz := machine.D10

	led3.Configure(machine.PinConfig{Mode: machine.PinOutput})
	led2.Configure(machine.PinConfig{Mode: machine.PinOutput})
	led1.Configure(machine.PinConfig{Mode: machine.PinOutput})
	bz.Configure(machine.PinConfig{Mode: machine.PinOutput})

	// 初期化
	led1.High()
	led2.High()
	led3.High()
	bz.Low()

	// BLEを有効
	err := adapter.Enable()
	if err != nil {
		return err
	}

	adv := adapter.DefaultAdvertisement()
	err = adv.Configure(bluetooth.AdvertisementOptions{
		LocalName: "tinygo ble peripheral",
	})
	if err != nil {
		return err
	}

	err = adv.Start()
	if err != nil {
		return err
	}

	var txChar bluetooth.Characteristic

	ch := make(chan []byte, 4)
	ch1 := make(chan []byte, 13)

	go getSensor(ch1)

	err = adapter.AddService(&bluetooth.Service{
		UUID: serviceUUID,
		Characteristics: []bluetooth.CharacteristicConfig{
			{
				UUID:  rxUUID,
				Flags: bluetooth.CharacteristicWriteWithoutResponsePermission | bluetooth.CharacteristicWritePermission,
				// | bluetooth.CharacteristicReadPermission
				WriteEvent: func(client bluetooth.Connection, offset int, value []byte) {
					if offset != 0 || len(value) != 1 {
						return
					}
					ch <- value
				},
			},
			{
				Handle: &txChar,
				UUID:   txUUID,
				Flags:  bluetooth.CharacteristicReadPermission | bluetooth.CharacteristicNotifyPermission,
			},
		},
	})

	if err != nil {
		return err
	}

	for {
		select {
		case val := <-ch:
			if (val[0] & 0x01) == 0x01 {
				led1.Low()

			} else {
				led1.High()
			}

			if (val[0] & 0x02) == 0x02 {
				led2.Low()

			} else {
				led2.High()
			}

			if (val[0] & 0x04) == 0x04 {
				led3.Low()

			} else {
				led3.High()
			}

			if (val[0] & 0x08) == 0x08 {
				bz.High()
			} else {
				bz.Low()
			}
			break

		case tData := <-ch1:
			txChar.Write(tData)
			break
		}
	}

	return nil
}
