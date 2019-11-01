package zmqreceivesendhandler

import (
	"encoding/json"
	"github.com/json-iterator/go"
	"strconv"
	"time"

	"github.com/conthing/export-homebridge/getedgexparams"
	"github.com/conthing/export-homebridge/homebridgeconfig"

	"github.com/conthing/utils/common"
	zmq "github.com/pebbe/zmq4"
)

const (
	CONTROLSTRING = "http://localhost:48082/api/v1/device/"
	HVACURL = "http://localhost:48082/api/v1/device/name/"
)

var EdgexToHomebridgeHvacModeMapOn = map[string]string{
	"AC":     "COOL",
	"HEATER": "HEAT",
	"VENT":   "AUTO",
	"DEHUMI": "AUTO",
}

var EdgexToHomebridgeHvacModeMapOff = map[string]string{
	"AC":     "OFF",
	"HEATER": "OFF",
	"VENT":   "OFF",
	"DEHUMI": "OFF",
}

//CommandZmq is the command from zmq
type CommandZmq struct {
	Name    string `json:"name"`  //注:Name是指alias
	Service string `json:"service"`
	ID      string `json:"id"`  //注:ID是指deviceid
	Command struct {
		Name   string `json:"name"`  //注:Name是指虚拟设备的一些特征值，诸如:onoff mode fanlevel ttarget percent brightnesss等
		Params interface{}
	} `json:"command"`
}
type Event struct {
	Device   string  //注:这个Device是真正的name
	Readings []Reading
}

//Reading means readings
type Reading struct {
	Name  string  //注:Name是指虚拟设备的一些特征值，诸如:onoff mode fanlevel ttarget percent brightnesss等
	Value string   //注:这个Value是特征名称的数值，诸如:0-100之间的整数 true false low middle high heat cool vent dehumi等
}
type DimmerableLightStatus struct {
	Id             string                      `json:"id"`
	Name           string                      `json:"name"`
	Service        string                      `json:"service"`
	Characteristic StDimmerLightCharacteristic `json:"characteristic"`
}
type StDimmerLightCharacteristic struct {
	Brightness int  `json:"brightness"`
	On         bool `json:"on"`
}
type LightStatus struct {
	Id             string                `json:"id"`
	Name           string                `json:"name"`
	Service        string                `json:"service"`
	Characteristic StLightCharacteristic `json:"characteristic"`
}
type StLightCharacteristic struct {
	On bool `json:"on"`
}
type CurtainStatus struct {
	Id             string                  `json:"id"`
	Name           string                  `json:"name"`
	Service        string                  `json:"service"`
	Characteristic StCurtainCharacteristic `json:"characteristic"`
}
type StCurtainCharacteristic struct {
	Percent int `json:"percent"`
}

//定义空调的状态结构体
type HvacStatus struct {
	Id             string               `json:"id"`
	Name           string               `json:"name"`
	Service        string               `json:"service"`
	Characteristic StHvacCharacteristic `json:"characteristic"`
}
type StHvacCharacteristic struct {
	On       bool   `json:"on"`
	Ttarget  int    `json:"ttarget"`
	Mode     string `json:"mode"`
	Fanlevel string `json:"fanlevel"`
}

var newPublisher *zmq.Socket
var Statuspubport string
var QRcode string

func InitZmq(statusport string) error {
	var err error
	newPublisher, err = zmq.NewSocket(zmq.PUB)
	if err != nil {
		common.Log.Errorf("InitZmq(statusport string) zmq.NewSocket(zmq.PUB) failed: %v", err)
		return err
	}
	Statuspubport = statusport
	common.Log.Info("zmq bind to ", statusport)
	_ = newPublisher.Bind(statusport)
	time.Sleep(200 * time.Millisecond) //休眠200ms
	return nil
}
func ZmqInit() error {
	context, err := zmq.NewContext()
	if err != nil {
		common.Log.Errorf("ZmqInit() zmq.NewContext() failed: %v", err)
		return err
	}
	commandRep, err := context.NewSocket(zmq.REP)
	if err != nil {
		common.Log.Errorf("ZmqInit() context.NewContext(zmq.REP) failed: %v", err)
		return err
	}
	defer func() {
		err = commandRep.Close()
		if err != nil {
			common.Log.Errorf("ZmqInit() commandRep.Close() failed: %v", err)
		}
	}()
	err = commandRep.Connect("tcp://127.0.0.1:9998")
	if err != nil {
		common.Log.Errorf("ZmqInit() commandRep.Connect(tcp://127.0.0.1:9998) failed: %v", err)
		return err
	}
	var commandzmq CommandZmq
	for {
		msg, err := commandRep.Recv(0) //recieve message by commandrep
		if err != nil {
			common.Log.Errorf("ZmqInit() commandRep.Recv(0) failed: %v", err)
			return err
		}
		msgbyte := []byte(msg)
		err = json.Unmarshal([]byte(msgbyte), &commandzmq)
		if err != nil {
			common.Log.Errorf("ZmqInit() msgbyte json.Unmarshal([]byte(msgbyte), &commandzmq) failed: %v", err)
		}
		common.Log.Info("Got: ", string(msg))
		_, err = commandRep.Send(msg, 0)
		if err != nil {
			common.Log.Errorf("ZmqInit() commandRep.Send(msg, 0) failed: %v", err)
			return err
		}
		if commandzmq.Command.Name == "init" {
			QRcode = commandzmq.Command.Params.(map[string]interface{})["QRcode"].(string)
			//value, ok := commandzmq.Command.Params.(map[string]interface{})["QRcode"].(string)
			//if !ok {
			//	return errors.QRCodeAssertErr
			//}
			//QRcode = value
			for i := range homebridgeconfig.Accessarysenders {
				var deviceID = homebridgeconfig.Accessarysenders[i].ID
				for n := range homebridgeconfig.Accessarysenders[i].Commands {
					var commandID = homebridgeconfig.Accessarysenders[i].Commands[n].ID
					coreCommandURL := commandform(commandID, deviceID)
					//common.Log.Info("coreCommandURL: ", coreCommandURL)
					result, err := getedgexparams.GetMessage(coreCommandURL)
					if err != nil {
						common.Log.Errorf("ZmqInit() getedgexparams.GetMessag(statuscommand) failed: %v", err)
					}
					if string(result) != "" {
						err = EventHanler(string(result))
						if err != nil {
							common.Log.Errorf("ZmqInit() EventHanler(string(result)) failed: %v", err)
						}
					}
				}
			}
		} else {
			getEdgexParams(commandzmq)
		}
	}
}
func getEdgexParams(commandzmq CommandZmq) {
	commandname := ""
	id := commandzmq.ID
	params := commandzmq.Command.Params
	common.Log.Info("params: ", params)
	data := make(map[string]string)
	if params.(map[string]interface{})["onOrOff"] != nil {
		onoroff := params.(map[string]interface{})["onOrOff"].(bool)
		data["onoff"] = strconv.FormatBool(onoroff)
		commandname = "onoff"
		go sendcommand(id, data, commandname)
	} else if params.(map[string]interface{})["percent"] != nil {
		percent := params.(map[string]interface{})["percent"].(float64)
		data["percent"] = strconv.FormatInt(int64(percent), 10)
		commandname = "percent"
		go sendcommand(id, data, commandname)
	} else if params.(map[string]interface{})["brightness"] != nil {
		brightness := params.(map[string]interface{})["brightness"].(float64)
		data["brightness"] = strconv.FormatInt(int64(brightness), 10)
		commandname = "brightness"
		go sendcommand(id, data, commandname)
	} else if params.(map[string]interface{})["t_target"] != nil {
		ttarget := params.(map[string]interface{})["t_target"].(float64)
		data["ttarget"] = strconv.FormatInt(int64(ttarget), 10)
		commandname = "ttarget"
		go sendcommand(id, data, commandname)
	} else if params.(map[string]interface{})["mode"] != nil {
		mode := params.(map[string]interface{})["mode"].(string)
		switch mode {
		case "HEAT":
			go sendcommand(id, map[string]string{"onoff": "true"}, "onoff")
			data["mode"] = "HEATER"
			commandname = "mode"
			go sendcommand(id, data, commandname)
		case "OFF":
			data["onoff"] = "false"
			commandname = "onoff"
			go sendcommand(id, data, commandname)
		case "COOL":
			go sendcommand(id, map[string]string{"onoff": "true"}, "onoff")
			data["mode"] = "AC"
			commandname = "mode"
			go sendcommand(id, data, commandname)
		case "AUTO":
			go sendcommand(id, map[string]string{"onoff": "true"}, "onoff")
			data["mode"] = "AUTO"
			commandname = "mode"
			go sendcommand(id, data, commandname)
		}
	} else if params.(map[string]interface{})["fanlevel"] != nil {
		fanlevel := params.(map[string]interface{})["fanlevel"].(string)
		data["fanlevel"] = string(fanlevel) //加的空调的风速设置fanlevel，fanlevel属性是string,输入string,输出也是string，fanlevel取值"LOW,MIDDLE,HIGH"
		commandname = "fanlevel"
		go sendcommand(id, data, commandname)
	} else {
		common.Log.Info("other type")
	}
}
func sendcommand(proxyid string, data map[string]string, commandname string) {
	datajson, err := json.Marshal(data)
	if err != nil {
		common.Log.Errorf("json.Marshal(data) failed: %v", err)
	}
	params := string(datajson)
	//common.Log.Debugf("sendcommand(%s, %s, %s)", proxyid, params, commandname)
	for j := range homebridgeconfig.Accessarysenders {
		deviceid := homebridgeconfig.Accessarysenders[j].ID
		if deviceid == proxyid {
			//common.Log.Info("deviceid: ", deviceid, commandname, params)
			for k := range homebridgeconfig.Accessarysenders[j].Commands {
				if homebridgeconfig.Accessarysenders[j].Commands[k].Name == commandname {
					switch commandname {
					case "brightness":
						commandid := homebridgeconfig.Accessarysenders[j].Commands[k].ID
						controlcommand := commandform(commandid, deviceid)
						result, err := getedgexparams.Put(controlcommand, params)
						if err != nil {
							common.Log.Errorf("sendcommand(proxyid string, params string, commandname string) case brightness getedgexparams.Put failed: %v", err)
						}
						common.Log.Info("put result", string(result))
					case "percent":
						commandid := homebridgeconfig.Accessarysenders[j].Commands[k].ID
						controlcommand := commandform(commandid, deviceid)
						result, err := getedgexparams.Put(controlcommand, params)
						if err != nil {
							common.Log.Errorf("sendcommand(proxyid string, params string, commandname string) case percent getedgexparams.Put failed: %v", err)
						}
						common.Log.Info("put result", string(result))
					case "onoff":
						commandid := homebridgeconfig.Accessarysenders[j].Commands[k].ID
						controlcommand := commandform(commandid, deviceid)
						result, err := getedgexparams.Put(controlcommand, params)
						if err != nil {
							common.Log.Errorf("sendcommand(proxyid string, params string, commandname string) case onoff getedgexparams.Put failed: %v", err)
						}
						common.Log.Info("put result", string(result))
					case "ttarget": //sendcommand加的空调的温度设置ttarget
						commandid := homebridgeconfig.Accessarysenders[j].Commands[k].ID
						controlcommand := commandform(commandid, deviceid)
						result, err := getedgexparams.Put(controlcommand, params)
						if err != nil {
							common.Log.Errorf("sendcommand(proxyid string, params string, commandname string) case ttarget getedgexparams.Put failed: %v", err)
						}
						common.Log.Info("put result", string(result))
					case "mode": //sendcommand加的空调的模式设置mode
						commandid := homebridgeconfig.Accessarysenders[j].Commands[k].ID
						controlcommand := commandform(commandid, deviceid)
						result, err := getedgexparams.Put(controlcommand, params)
						if err != nil {
							common.Log.Errorf("sendcommand(proxyid string, params string, commandname string) case mode getedgexparams.Put failed: %v", err)
						}
						common.Log.Info("put result", string(result))
					case "fanlevel": //sendcommand加的空调的风速设置fanlevel
						commandid := homebridgeconfig.Accessarysenders[j].Commands[k].ID
						controlcommand := commandform(commandid, deviceid)
						result, err := getedgexparams.Put(controlcommand, params)
						if err != nil {
							common.Log.Errorf("sendcommand(proxyid string, params string, commandname string) case fanlevel getedgexparams.Put failed: %v", err)
						}
						common.Log.Info("put result", string(result))
					default:
						common.Log.Info("in default")
					}
					return
				}
			}
			common.Log.Errorf("command %s not found, commands %+v", commandname, homebridgeconfig.Accessarysenders[j].Commands)
			return
		}
	}
	common.Log.Errorf("proxyid %s not found, accsender %+v", proxyid, homebridgeconfig.Accessarysenders)
}
func commandform(commandid string, deviceid string) string {
	controlcommand := CONTROLSTRING + deviceid + "/command/" + commandid
	return controlcommand
}
//以下代码是发给homebridge的
func EventHanler(bd string) (err error) {
	var event Event
	var status map[string]interface{}
	status = make(map[string]interface{})
	common.Log.Info("收到的event： ", bd)
	bytestr := []byte(bd)
	err = json.Unmarshal([]byte(bytestr), &event)
	if err != nil {
		common.Log.Errorf("EventHanler(bd string) bytestr json.Umarshal([]byte(bytestr), &event) failed: %v", err)
	}
	devicename := event.Device
	for i := range homebridgeconfig.Accessaries {
		defaultname := homebridgeconfig.Accessarysenders[i].Name
		defaultid := homebridgeconfig.Accessaries[i].ProxyID
		defaulttype := homebridgeconfig.Accessaries[i].Service
		if devicename == defaultname {
			var dimmerablelightstatus DimmerableLightStatus
			var curtainstatus CurtainStatus
			var lightstatus LightStatus
			var hvacstatus HvacStatus
			for j := range event.Readings {
				switch event.Readings[j].Name {
				case "brightness":
					if homebridgeconfig.Accessaries[i].Dimmerable == "true" {
						dimmerablelightstatus.Characteristic.Brightness, _ = strconv.Atoi(event.Readings[j].Value)
						if dimmerablelightstatus.Characteristic.Brightness > 0 {
							dimmerablelightstatus.Characteristic.On = true
						} else {
							dimmerablelightstatus.Characteristic.On = false
						}
						dimmerablelightstatus.Id = defaultid
						dimmerablelightstatus.Name = defaultname
						dimmerablelightstatus.Service = defaulttype
						status["status"] = dimmerablelightstatus
					}
				case "percent":
					curtainstatus.Characteristic.Percent, _ = strconv.Atoi(event.Readings[j].Value)
					curtainstatus.Id = defaultid
					curtainstatus.Name = defaultname
					curtainstatus.Service = defaulttype
					status["status"] = curtainstatus
				case "onoff":
					if defaulttype == "Lightbulb"{
						lightstatus.Characteristic.On, _ = strconv.ParseBool(event.Readings[j].Value)
						lightstatus.Id = defaultid
						lightstatus.Name = defaultname
						lightstatus.Service = defaulttype
						status["status"] = lightstatus
					}else if defaulttype == "Thermostat"{
						if event.Readings[j].Value == "false"{
							//hvacstatus.Characteristic.Mode = "OFF"
							hvacstatus.Characteristic.Mode = EdgexToHomebridgeHvacModeMapOff[event.Readings[j].Value]
						}else if event.Readings[j].Value == "true"{
							content, err := getedgexparams.GetMessage(HVACURL + devicename)
							if err != nil {
								common.Log.Error("EventHanler(bd string) case onoff getedgexparams.GetMessage(HVACURL + event.Device) failed: ", err)
								return err
							}
							id := jsoniter.Get(content,"id").ToString()
							if id == "" {
                                common.Log.Warn("EventHanler(bd string) case onoff id failed: ", id)
							}
                            url := FindSingleDeviceCommandsMode(content, id)
                            if url == ""{
                            	common.Log.Warn("EventHanler(bd string) case onoff url failed: ", url)
							}
                            data, err := getedgexparams.GetMessage(url)
                            if err != nil {
                            	common.Log.Error("EventHanler(bd string) case onoff err failed: ", err)
							}
                            common.Log.Info("单个设备的mode信息: ", string(data))
                            modevalue := jsoniter.Get(data, "readings", 0, "value").ToString()
                            hvacstatus.Characteristic.Mode = EdgexToHomebridgeHvacModeMapOn[modevalue]
                            common.Log.Info("单个设备的modevalue信息: ", modevalue)
						}
						hvacstatus.Id = defaultid
						hvacstatus.Name = defaultname
						hvacstatus.Service = defaulttype
						status["status"] = hvacstatus
					}
				case "ttarget":
					hvacstatus.Characteristic.Ttarget, _ = strconv.Atoi(event.Readings[j].Value)
					hvacstatus.Id = defaultid
					hvacstatus.Name = defaultname
					hvacstatus.Service = defaulttype
					status["status"] = hvacstatus
				case "mode":
					hvacstatus.Characteristic.Mode = EdgexToHomebridgeHvacModeMapOn[event.Readings[j].Value]
					hvacstatus.Id = defaultid
					hvacstatus.Name = defaultname
					hvacstatus.Service = defaulttype
					status["status"] = hvacstatus
				case "fanlevel":
					hvacstatus.Characteristic.Fanlevel = event.Readings[j].Value
					hvacstatus.Id = defaultid
					hvacstatus.Name = defaultname
					hvacstatus.Service = defaulttype
					status["status"] = hvacstatus
				default:
					return
				}
			}
		}
	}
	data, err := json.MarshalIndent(status, "", " ")
	if err != nil {
		common.Log.Errorf("EventHanler(bd string) data json.MarshalIndent failed: %v", err)
	}
	if string(data) != "{}" {
		common.Log.Info("send to js ", string(data))
		if newPublisher != nil {
			_, err = newPublisher.SendMessage("status", data)
		}
	}
	return
}

// FindSingleDeviceCommands 针对 GETDEVICEBYNAMEURL 获取commands
func FindSingleDeviceCommandsMode(content []byte, id string) string {
	var device homebridgeconfig.EdgexCommandDevice
	jsoniter.Get(content).ToVal(&device)
	common.Log.Info(device)
	for _, command := range device.Commands {
		if command.Name == "mode" {
			return command.GET.URL
		}
	}
	return ""
}

//func FindSingleDeviceCommandsOnoff(content []byte, id string) string {
//	var device homebridgeconfig.EdgexCommandDevice
//	jsoniter.Get(content).ToVal(&device)
//	common.Log.Info(device)
//	for _, command := range device.Commands {
//		if command.Name == "onoff" {
//			return command.GET.URL
//		}
//	}
//	return ""
//}
