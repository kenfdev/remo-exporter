package exporter

import (
	"fmt"
	"strconv"

	"github.com/kenfdev/remo-exporter/types"
)

const (
	EpcNormalDirectionCumulativeElectricEnergy  = 224
	EpcReverseDirectionCumulativeElectricEnergy = 227
	EpcCoefficient                              = 211
	EpcCumulativeElectricEnergyUnit             = 225
	EpcCumulativeElectricEnergyEffectiveDigits  = 215
	EpcMeasuredInstantaneous                    = 231
)

type EnergyInfo struct {
	NormalEnergy          int
	ReverseEnergy         int
	Coefficient           int
	EnergyUnit            float64
	EffectiveDigits       int
	MeasuredInstantaneous int
}

func (i EnergyInfo) CumulativeElectricEnergy() float64 {
	return float64((i.NormalEnergy-i.ReverseEnergy)*i.Coefficient) * i.EnergyUnit
}

func getSmartMeters(apps []*types.Appliance) []*types.Appliance {
	smartMeters := make([]*types.Appliance, 0)
	for _, app := range apps {
		if app.Type == "EL_SMART_METER" {
			smartMeters = append(smartMeters, app)
		}
	}
	return smartMeters
}

func energyInfo(sm *types.Appliance) (*EnergyInfo, error) {
	if sm.SmartMeter == nil {
		return nil, fmt.Errorf("'%s' does not have smart_meter field", sm.Device.Name)
	}
	if len(sm.SmartMeter.EchonetliteProperties) != 6 {
		return nil, fmt.Errorf("'%s' has incorrect echonetlite_properties", sm.Device.Name)
	}
	var info EnergyInfo
	var err error
	for _, p := range sm.SmartMeter.EchonetliteProperties {
		switch p.Epc {
		case EpcNormalDirectionCumulativeElectricEnergy:
			info.NormalEnergy, err = strconv.Atoi(p.Val)
			if err != nil {
				return nil, err
			}
		case EpcReverseDirectionCumulativeElectricEnergy:
			info.ReverseEnergy, err = strconv.Atoi(p.Val)
			if err != nil {
				return nil, err
			}
		case EpcCoefficient:
			info.Coefficient, err = strconv.Atoi(p.Val)
			if err != nil {
				return nil, err
			}
		case EpcCumulativeElectricEnergyUnit:
			unit, err := strconv.Atoi(p.Val)
			if err != nil {
				return nil, err
			}
			switch unit {
			case 0:
				info.EnergyUnit = 1
			case 1:
				info.EnergyUnit = 0.1
			case 2:
				info.EnergyUnit = 0.01
			case 3:
				info.EnergyUnit = 0.001
			case 4:
				info.EnergyUnit = 0.0001
			case 10:
				info.EnergyUnit = 10
			case 11:
				info.EnergyUnit = 100
			case 12:
				info.EnergyUnit = 1000
			case 13:
				info.EnergyUnit = 10000
			default:
				return nil, fmt.Errorf("invalid CumulativeElectricEnergyUnit value: %d", unit)
			}
		case EpcCumulativeElectricEnergyEffectiveDigits:
			info.EffectiveDigits, err = strconv.Atoi(p.Val)
			if err != nil {
				return nil, err
			}
		case EpcMeasuredInstantaneous:
			info.MeasuredInstantaneous, err = strconv.Atoi(p.Val)
			if err != nil {
				return nil, err
			}
		}
	}
	return &info, nil
}
