package db

import (
	"blockbook/bchain"
	"bytes"
	"math/big"
	"github.com/golang/glog"
	"github.com/syscoin/btcd/wire"
	"github.com/juju/errors"
)

func (d *RocksDB) ConnectAssetOutput(sptData []byte, balances map[string]*bchain.AddrBalance, version int32, addresses bchain.AddressesMap, btxID []byte, outputIndex int32) error {
	r := bytes.NewReader(sptData)
	var asset wire.AssetType
	err := asset.Deserialize(r)
	if err != nil {
		return err
	}
	assetGuid := asset.Asset
	assetSenderAddrDesc, err := d.chainParser.GetAddrDescFromAddress(asset.WitnessAddress.ToString("sys"))
	if err != nil || len(assetSenderAddrDesc) == 0 || len(assetSenderAddrDesc) > maxAddrDescLen {
		if err != nil {
			// do not log ErrAddressMissing, transactions can be without to address (for example eth contracts)
			if err != bchain.ErrAddressMissing {
				glog.Warningf("ConnectAssetOutput sender with asset %v (%v) could not be decoded error %v", assetGuid, string(assetSenderAddrDesc), err)
			}
		} else {
			glog.Warningf("ConnectAssetOutput sender with asset %v (%v) has invalid length: %d", assetGuid, string(assetSenderAddrDesc), len(assetSenderAddrDesc))
		}
		return errors.New("ConnectAssetOutput Skipping asset tx")
	}
	senderStr := string(assetSenderAddrDesc)
	balance, e := balances[senderStr]
	if !e {
		balance, err = d.GetAddrDescBalance(assetSenderAddrDesc, bchain.AddressBalanceDetailUTXOIndexed)
		if err != nil {
			return err
		}
		if balance == nil {
			balance = &bchain.AddrBalance{}
		}
		balances[senderStr] = balance
		d.cbs.balancesMiss++
	} else {
		d.cbs.balancesHit++
	}

	counted := addToAddressesMap(addresses, senderStr, btxID, outputIndex)
	if !counted {
		balance.Txs++
	}

	if len(asset.WitnessAddressTransfer.WitnessProgram) > 0 {
		assetTransferWitnessAddrDesc, err := d.chainParser.GetAddrDescFromAddress(asset.WitnessAddressTransfer.ToString("sys"))
		if err != nil || len(assetSenderAddrDesc) == 0 || len(assetSenderAddrDesc) > maxAddrDescLen {
			if err != nil {
				// do not log ErrAddressMissing, transactions can be without to address (for example eth contracts)
				if err != bchain.ErrAddressMissing {
					glog.Warningf("ConnectAssetOutput transferee with asset %v (%v) could not be decoded error %v", assetGuid, string(assetTransferWitnessAddrDesc), err)
				}
			} else {
				glog.Warningf("ConnectAssetOutput transferee with asset %v (%v) has invalid length: %d", assetGuid, string(assetTransferWitnessAddrDesc), len(assetTransferWitnessAddrDesc))
			}
			return errors.New("ConnectAssetOutput Skipping asset transfer tx")
		}
		glog.Warningf("transfering asset %v to %v from %v", assetGuid, asset.WitnessAddressTransfer.ToString("sys"), asset.WitnessAddress.ToString("sys"))
		transferStr := string(assetTransferWitnessAddrDesc)
		balanceTransfer, e := balances[transferStr]
		if !e {
			balanceTransfer, err = d.GetAddrDescBalance(assetTransferWitnessAddrDesc, bchain.AddressBalanceDetailUTXOIndexed)
			if err != nil {
				return err
			}
			if balance == nil {
				balanceTransfer = &bchain.AddrBalance{}
			}
			balances[transferStr] = balanceTransfer
			d.cbs.balancesMiss++
		} else {
			d.cbs.balancesHit++
		}
		counted := addToAddressesMap(addresses, transferStr, btxID, outputIndex)
		if !counted {
			balanceTransfer.Txs++
		}
		// transfer balance from old address to transfered address
		if balanceTransfer.BalanceAssetUnAllocatedSat == nil{
			balanceTransfer.BalanceAssetUnAllocatedSat = map[uint32]*big.Int{}
		}
		valueSat := balance.BalanceAssetUnAllocatedSat[assetGuid]
		balanceTransfer.BalanceAssetUnAllocatedSat[assetGuid] = big.NewInt(valueSat.Int64())
		valueSat.Set(big.NewInt(0))
		glog.Warningf("transfer done asset %v to %v from %v new balance on sender %v", assetGuid, asset.WitnessAddressTransfer.ToString("sys"), asset.WitnessAddress.ToString("sys"), balance.BalanceAssetUnAllocatedSat[assetGuid])
	} else {
		if balance.BalanceAssetUnAllocatedSat == nil{
			balance.BalanceAssetUnAllocatedSat = map[uint32]*big.Int{}
		}
		balanceAssetUnAllocatedSat, ok := balance.BalanceAssetUnAllocatedSat[assetGuid]
		if !ok {
			balanceAssetUnAllocatedSat = big.NewInt(0)
			balance.BalanceAssetUnAllocatedSat[assetGuid] = balanceAssetUnAllocatedSat
		}
		valueSat := big.NewInt(asset.Balance)
		balanceAssetUnAllocatedSat.Add(balanceAssetUnAllocatedSat, valueSat)
	}
	return nil
}

func (d *RocksDB) ConnectAssetAllocationOutput(sptData []byte, balances map[string]*bchain.AddrBalance, version int32, addresses bchain.AddressesMap, btxID []byte, outputIndex int32) error {
	r := bytes.NewReader(sptData)
	var assetAllocation wire.AssetAllocationType
	err := assetAllocation.Deserialize(r)
	if err != nil {
		return err
	}
	
	totalAssetSentValue := big.NewInt(0)
	assetGuid := assetAllocation.AssetAllocationTuple.Asset
	assetSenderAddrDesc, err := d.chainParser.GetAddrDescFromAddress(assetAllocation.AssetAllocationTuple.WitnessAddress.ToString("sys"))
	if err != nil || len(assetSenderAddrDesc) == 0 || len(assetSenderAddrDesc) > maxAddrDescLen {
		if err != nil {
			// do not log ErrAddressMissing, transactions can be without to address (for example eth contracts)
			if err != bchain.ErrAddressMissing {
				glog.Warningf("ConnectAssetAllocationOutput sender with asset %v (%v) could not be decoded error %v", assetGuid, string(assetSenderAddrDesc), err)
			}
		} else {
			glog.Warningf("ConnectAssetAllocationOutput sender with asset %v (%v) has invalid length: %d", assetGuid, string(assetSenderAddrDesc), len(assetSenderAddrDesc))
		}
		return errors.New("ConnectAssetAllocationOutput Skipping asset allocation tx")
	}
	for _, allocation := range assetAllocation.ListSendingAllocationAmounts {
		addrDesc, err := d.chainParser.GetAddrDescFromAddress(allocation.WitnessAddress.ToString("sys"))
		if err != nil || len(addrDesc) == 0 || len(addrDesc) > maxAddrDescLen {
			if err != nil {
				// do not log ErrAddressMissing, transactions can be without to address (for example eth contracts)
				if err != bchain.ErrAddressMissing {
					glog.Warningf("ConnectAssetAllocationOutput receiver with asset %v (%v) could not be decoded error %v", assetGuid, string(addrDesc), err)
				}
			} else {
				glog.Warningf("ConnectAssetAllocationOutput receiver with asset %v (%v) has invalid length: %d", assetGuid, string(addrDesc), len(addrDesc))
			}
			continue
		}
		receiverStr := string(addrDesc)
		balance, e := balances[receiverStr]
		if !e {
			balance, err = d.GetAddrDescBalance(addrDesc, bchain.AddressBalanceDetailUTXOIndexed)
			if err != nil {
				return err
			}
			if balance == nil {
				balance = &bchain.AddrBalance{}
			}
			balances[receiverStr] = balance
			d.cbs.balancesMiss++
		} else {
			d.cbs.balancesHit++
		}

		// for each address returned, add it to map
		counted := addToAddressesMap(addresses, receiverStr, btxID, outputIndex)
		if !counted {
			balance.Txs++
		}

		if balance.BalanceAssetAllocatedSat == nil {
			balance.BalanceAssetAllocatedSat = map[uint32]*big.Int{}
		}
		balanceAssetAllocatedSat, ok := balance.BalanceAssetAllocatedSat[assetGuid]
		if !ok {
			balanceAssetAllocatedSat = big.NewInt(0)
			balance.BalanceAssetAllocatedSat[assetGuid] = balanceAssetAllocatedSat
		}
		amount := big.NewInt(allocation.ValueSat)
		balanceAssetAllocatedSat.Add(balanceAssetAllocatedSat, amount)
		totalAssetSentValue.Add(totalAssetSentValue, amount)
	}
	return d.ConnectAssetAllocationInput(btxID, assetGuid, version, totalAssetSentValue, assetSenderAddrDesc, balances)
}

func (d *RocksDB) DisconnectAssetAllocationOutput(sptData []byte, balances map[string]*bchain.AddrBalance, version int32, addresses map[string]struct{}) error {
	r := bytes.NewReader(sptData)
	var assetAllocation wire.AssetAllocationType
	err := assetAllocation.Deserialize(r)
	if err != nil {
		return err
	}
	getAddressBalance := func(addrDesc bchain.AddressDescriptor) (*bchain.AddrBalance, error) {
		var err error
		s := string(addrDesc)
		b, fb := balances[s]
		if !fb {
			b, err = d.GetAddrDescBalance(addrDesc, bchain.AddressBalanceDetailUTXOIndexed)
			if err != nil {
				return nil, err
			}
			balances[s] = b
		}
		return b, nil
	}
	totalAssetSentValue := big.NewInt(0)
	assetGuid := assetAllocation.AssetAllocationTuple.Asset
	assetSenderAddrDesc, err := d.chainParser.GetAddrDescFromAddress(assetAllocation.AssetAllocationTuple.WitnessAddress.ToString("sys"))
	if err != nil || len(assetSenderAddrDesc) == 0 || len(assetSenderAddrDesc) > maxAddrDescLen {
		if err != nil {
			// do not log ErrAddressMissing, transactions can be without to address (for example eth contracts)
			if err != bchain.ErrAddressMissing {
				glog.Warningf("DisconnectAssetAllocationOutput sender with asset %v (%v) could not be decoded error %v", assetGuid, string(assetSenderAddrDesc), err)
			}
		} else {
			glog.Warningf("DisconnectAssetAllocationOutput sender with asset %v (%v) has invalid length: %d", assetGuid, string(assetSenderAddrDesc), len(assetSenderAddrDesc))
		}
		return errors.New("DisconnectAssetAllocationOutput Skipping disconnect asset allocation tx")
	}
	for _, allocation := range assetAllocation.ListSendingAllocationAmounts {
		addrDesc, err := d.chainParser.GetAddrDescFromAddress(allocation.WitnessAddress.ToString("sys"))
		if err != nil || len(addrDesc) == 0 || len(addrDesc) > maxAddrDescLen {
			if err != nil {
				// do not log ErrAddressMissing, transactions can be without to address (for example eth contracts)
				if err != bchain.ErrAddressMissing {
					glog.Warningf("DisconnectAssetAllocationOutput receiver with asset %v (%v) could not be decoded error %v", assetGuid, string(addrDesc), err)
				}
			} else {
				glog.Warningf("DisconnectAssetAllocationOutput receiver with asset %v (%v) has invalid length: %d", assetGuid, string(addrDesc), len(addrDesc))
			}
			continue
		}
		receiverStr := string(addrDesc)
		_, exist := addresses[receiverStr]
		if !exist {
			addresses[receiverStr] = struct{}{}
		}
		balance, err := getAddressBalance(addrDesc)
		if err != nil {
			return err
		}
		if balance != nil {
			// subtract number of txs only once
			if !exist {
				balance.Txs--
			}
		} else {
			ad, _, _ := d.chainParser.GetAddressesFromAddrDesc(addrDesc)
			glog.Warningf("DisconnectAssetAllocationOutput Balance for asset address %v (%v) not found", ad, addrDesc)
		}

		if balance.BalanceAssetAllocatedSat != nil{
			balanceAssetAllocatedSat := balance.BalanceAssetAllocatedSat[assetGuid]
			amount := big.NewInt(allocation.ValueSat)
			balanceAssetAllocatedSat.Sub(balanceAssetAllocatedSat, amount)
			if balanceAssetAllocatedSat.Sign() < 0 {
				d.resetValueSatToZero(balanceAssetAllocatedSat, addrDesc, "balance")
			}
			totalAssetSentValue.Add(totalAssetSentValue, amount)
		} else {
			ad, _, _ := d.chainParser.GetAddressesFromAddrDesc(addrDesc)
			glog.Warningf("DisconnectAssetAllocationOutput Asset Balance for asset address %v (%v) not found", ad, addrDesc)
		}
	}
	return d.DisconnectAssetAllocationInput(assetGuid, version, totalAssetSentValue, assetSenderAddrDesc, balances)
}

func (d *RocksDB) ConnectAssetAllocationInput(btxID []byte, assetGuid uint32, version int32, totalAssetSentValue *big.Int, assetSenderAddrDesc bchain.AddressDescriptor, balances map[string]*bchain.AddrBalance) error {
	assetStrSenderAddrDesc := string(assetSenderAddrDesc)
	balance, e := balances[assetStrSenderAddrDesc]
	if !e {
		balance, err := d.GetAddrDescBalance(assetSenderAddrDesc, bchain.AddressBalanceDetailUTXOIndexed)
		if err != nil {
			return err
		}
		if balance == nil {
			balance = &bchain.AddrBalance{}
		}
		balances[assetStrSenderAddrDesc] = balance
		d.cbs.balancesMiss++
	} else {
		d.cbs.balancesHit++
	}
	if d.chainParser.IsSyscoinAssetSend(version) {
		if balance.SentAssetUnAllocatedSat == nil {
			balance.SentAssetUnAllocatedSat = map[uint32]*big.Int{}
		}
		sentAssetUnAllocatedSat, ok := balance.SentAssetUnAllocatedSat[assetGuid]
		if !ok {
			sentAssetUnAllocatedSat = big.NewInt(0)
			balance.SentAssetUnAllocatedSat[assetGuid] = sentAssetUnAllocatedSat
		}
		balanceAssetUnAllocatedSat := balance.BalanceAssetUnAllocatedSat[assetGuid]
		balanceAssetUnAllocatedSat.Sub(balanceAssetUnAllocatedSat, totalAssetSentValue)
		sentAssetUnAllocatedSat.Add(sentAssetUnAllocatedSat, totalAssetSentValue)
		if balanceAssetUnAllocatedSat.Sign() < 0 {
			d.resetValueSatToZero(balanceAssetUnAllocatedSat, assetSenderAddrDesc, "balance")
		}

	} else {
		if balance.SentAssetAllocatedSat == nil {
			balance.SentAssetAllocatedSat = map[uint32]*big.Int{}
		}
		sentAssetAllocatedSat, ok := balance.SentAssetAllocatedSat[assetGuid]
		if !ok {
			sentAssetAllocatedSat = big.NewInt(0)
			balance.SentAssetAllocatedSat[assetGuid] = sentAssetAllocatedSat
		}
		balanceAssetAllocatedSat := balance.BalanceAssetAllocatedSat[assetGuid]
		balanceAssetAllocatedSat.Sub(balanceAssetAllocatedSat, totalAssetSentValue)
		sentAssetAllocatedSat.Add(sentAssetAllocatedSat, totalAssetSentValue)
		if balanceAssetAllocatedSat.Sign() < 0 {
			d.resetValueSatToZero(balanceAssetAllocatedSat, assetSenderAddrDesc, "balance")
		}
	}
	return nil

}

func (d *RocksDB) DisconnectAssetOutput(sptData []byte, balances map[string]*bchain.AddrBalance, version int32, addresses map[string]struct{}) error {
	r := bytes.NewReader(sptData)
	var asset wire.AssetType
	err := asset.Deserialize(r)
	if err != nil {
		return err
	}
	getAddressBalance := func(addrDesc bchain.AddressDescriptor) (*bchain.AddrBalance, error) {
		var err error
		s := string(addrDesc)
		b, fb := balances[s]
		if !fb {
			b, err = d.GetAddrDescBalance(addrDesc, bchain.AddressBalanceDetailUTXOIndexed)
			if err != nil {
				return nil, err
			}
			balances[s] = b
		}
		return b, nil
	}
	assetGuid := asset.Asset
	assetSenderAddrDesc, err := d.chainParser.GetAddrDescFromAddress(asset.WitnessAddress.ToString("sys"))
	assetStrSenderAddrDesc := string(assetSenderAddrDesc)
	_, exist := addresses[assetStrSenderAddrDesc]
	if !exist {
		addresses[assetStrSenderAddrDesc] = struct{}{}
	}
	balance, err := getAddressBalance(assetSenderAddrDesc)
	if err != nil {
		return err
	}
	if balance != nil {
		// subtract number of txs only once
		if !exist {
			balance.Txs--
		}
	} else {
		ad, _, _ := d.chainParser.GetAddressesFromAddrDesc(assetSenderAddrDesc)
		glog.Warningf("DisconnectAssetOutput Balance for asset address %s (%s) not found", ad, assetSenderAddrDesc)
	}
	if len(asset.WitnessAddressTransfer.WitnessProgram) > 0 {
		assetTransferWitnessAddrDesc, err := d.chainParser.GetAddrDescFromAddress(asset.WitnessAddressTransfer.ToString("sys"))
		transferStr := string(assetTransferWitnessAddrDesc)
		_, exist := addresses[transferStr]
		if !exist {
			addresses[transferStr] = struct{}{}
		}
		balanceTransfer, err := getAddressBalance(assetTransferWitnessAddrDesc)
		if err != nil {
			return err
		}
		if balanceTransfer != nil {
			// subtract number of txs only once
			if !exist {
				balanceTransfer.Txs--
			}
		} else {
			ad, _, _ := d.chainParser.GetAddressesFromAddrDesc(assetTransferWitnessAddrDesc)
			glog.Warningf("DisconnectAssetOutput Balance for transfer asset address %s (%s) not found", ad, assetTransferWitnessAddrDesc)
		}
		// transfer values back to original owner and 0 out the
		valueSat := balance.BalanceAssetUnAllocatedSat[assetGuid]
		balanceTransfer.BalanceAssetUnAllocatedSat[assetGuid] = big.NewInt(valueSat.Int64())
		valueSat.Set(big.NewInt(0))
		
	} else if balance.SentAssetUnAllocatedSat != nil {
		sentAssetUnAllocatedSat := balance.SentAssetUnAllocatedSat[assetGuid]
		balanceAssetUnAllocatedSat := balance.BalanceAssetUnAllocatedSat[assetGuid]
		valueSat := big.NewInt(asset.Balance)
		balanceAssetUnAllocatedSat.Add(balanceAssetUnAllocatedSat, valueSat)
		sentAssetUnAllocatedSat.Sub(sentAssetUnAllocatedSat, valueSat)
		if sentAssetUnAllocatedSat.Sign() < 0 {
			d.resetValueSatToZero(sentAssetUnAllocatedSat, assetSenderAddrDesc, "balance")
		}
	} else {
		glog.Warningf("DisconnectAssetOutput: Asset Sent balance not found guid %v (%v)", assetGuid, assetStrSenderAddrDesc)
	}
	return nil

}
func (d *RocksDB) DisconnectAssetAllocationInput(assetGuid uint32, version int32, totalAssetSentValue *big.Int, assetSenderAddrDesc bchain.AddressDescriptor, balances map[string]*bchain.AddrBalance) error {
	assetStrSenderAddrDesc := string(assetSenderAddrDesc)
	balance, e := balances[assetStrSenderAddrDesc]
	if !e {
		balance, err := d.GetAddrDescBalance(assetSenderAddrDesc, bchain.AddressBalanceDetailUTXOIndexed)
		if err != nil {
			return err
		}
		if balance == nil {
			return errors.New("DisconnectAssetAllocationInput Asset Balance for sender address not found")
		}
		balances[assetStrSenderAddrDesc] = balance
	}
	if d.chainParser.IsSyscoinAssetSend(version) {
		if balance.SentAssetUnAllocatedSat != nil {
			sentAssetUnAllocatedSat := balance.SentAssetUnAllocatedSat[assetGuid]
			balanceAssetUnAllocatedSat := balance.BalanceAssetUnAllocatedSat[assetGuid]
			balanceAssetUnAllocatedSat.Add(balanceAssetUnAllocatedSat, totalAssetSentValue)
			sentAssetUnAllocatedSat.Sub(sentAssetUnAllocatedSat, totalAssetSentValue)
			if sentAssetUnAllocatedSat.Sign() < 0 {
				d.resetValueSatToZero(sentAssetUnAllocatedSat, assetSenderAddrDesc, "balance")
			}

		} else {
			glog.Warningf("DisconnectAssetAllocationInput: AssetSend SentUnAllocated balance not found guid %v (%v)", assetGuid, assetStrSenderAddrDesc)
		}
	} else if balance.SentAssetAllocatedSat != nil {
		sentAssetAllocatedSat := balance.SentAssetAllocatedSat[assetGuid]
		balanceAssetAllocatedSat := balance.BalanceAssetAllocatedSat[assetGuid]
		balanceAssetAllocatedSat.Add(balanceAssetAllocatedSat, totalAssetSentValue)
		sentAssetAllocatedSat.Sub(sentAssetAllocatedSat, totalAssetSentValue)
		if sentAssetAllocatedSat.Sign() < 0 {
			d.resetValueSatToZero(sentAssetAllocatedSat, assetSenderAddrDesc, "balance")
		}

	} else {
		glog.Warningf("DisconnectAssetAllocationInput: Asset Sent Allocated balance not found guid %v (%v)", assetGuid, assetStrSenderAddrDesc)
	}
	return nil

}
func (d *RocksDB) ConnectSyscoinOutputs(addrDesc bchain.AddressDescriptor, balances map[string]*bchain.AddrBalance, version int32, addresses bchain.AddressesMap, btxID []byte, outputIndex int32) error {
	script, err := d.chainParser.GetScriptFromAddrDesc(addrDesc)
	if err != nil {
		return err
	}
	sptData := d.chainParser.TryGetOPReturn(script)
	if sptData == nil {
		return nil
	}
	if d.chainParser.IsAssetAllocationTx(version) {
		return d.ConnectAssetAllocationOutput(sptData, balances, version, addresses, btxID, outputIndex)
	} else if d.chainParser.IsAssetTx(version) {
		return d.ConnectAssetOutput(sptData, balances, version, addresses, btxID, outputIndex)
	}
	return nil
}

func (d *RocksDB) DisconnectSyscoinOutputs(addrDesc bchain.AddressDescriptor, balances map[string]*bchain.AddrBalance, version int32, addresses map[string]struct{}) error {
	script, err := d.chainParser.GetScriptFromAddrDesc(addrDesc)
	if err != nil {
		return err
	}
	sptData := d.chainParser.TryGetOPReturn(script)
	if sptData == nil {
		return nil
	}
	if d.chainParser.IsAssetAllocationTx(version) {
		return d.DisconnectAssetAllocationOutput(sptData, balances, version, addresses)
	} else if d.chainParser.IsAssetTx(version) {
		return d.DisconnectAssetOutput(sptData, balances, version, addresses)
	}
	return nil
}
