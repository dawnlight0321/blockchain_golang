package block

import (
	"bytes"
	"encoding/gob"
	"errors"
	"github.com/boltdb/bolt"
	"myCode/public_blockchain/part7-network/database"
	log "myCode/public_blockchain/part7-network/logcustom"
)

type UTXOHandle struct {
	BC *blockchain
}

//重置UTXO数据库
func (u *UTXOHandle) ResetUTXODataBase() {
	//先查找全部未花费UTXO
	utxosMap := u.BC.findAllUTXOs()
	//删除旧的UTXO数据库
	if database.IsBucketExist(u.BC.BD, database.UTXOBucket) {
		u.BC.BD.DeleteBucket(database.UTXOBucket)
	}
	//创建并将未花费UTXO循环添加
	for k, v := range utxosMap {
		u.BC.BD.Put([]byte(k), u.serialize(v), database.UTXOBucket)
	}
}

func (u *UTXOHandle) findUTXOFromAddress(address string) []*UTXO {
	publicKeyHash := getPublicKeyHashFromAddress(address)
	utxosSlice := []*UTXO{}
	//获取bolt迭代器，遍历整个UTXO数据库
	err := u.BC.BD.DB.View(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys
		b := tx.Bucket([]byte(database.UTXOBucket))
		if b == nil {
			return errors.New("datebase view err: not find bucket ")
		}
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			utxos := u.dserialize(v)
			for _, utxo := range utxos {
				if bytes.Equal(utxo.Vout.PublicKeyHash, publicKeyHash) {
					utxosSlice = append(utxosSlice, utxo)
				}
			}
		}
		return nil
	})
	if err != nil {
		log.Panic(err)
	}
	return utxosSlice
}

func (u *UTXOHandle) synchrodata(tss []Transaction) {
	//先将全部输入插入数据库
	for _, ts := range tss {
		utxos := []*UTXO{}
		for index, vOut := range ts.Vout {
			utxos = append(utxos, &UTXO{ts.TxHash, index, vOut})
		}
		u.BC.BD.Put(ts.TxHash, u.serialize(utxos), database.UTXOBucket)
	}

	//在用输出进行剔除
	for _, ts := range tss {
		for _, vIn := range ts.Vint {
			publicKeyHash := generatePublicKeyHash(vIn.PublicKey)
			//获取bolt迭代器，遍历整个UTXO数据库
			utxoByte := u.BC.BD.View(vIn.TxHash, database.UTXOBucket)
			if utxoByte == nil {
				log.Panic("Synchrodata err : do not find utxo")
			}
			utxos := u.dserialize(utxoByte)
			newUTXO := []*UTXO{}
			for _, utxo := range utxos {
				if utxo.Index == vIn.Index && bytes.Equal(utxo.Vout.PublicKeyHash, publicKeyHash) {
					continue
				}
				newUTXO = append(newUTXO, utxo)
			}
			u.BC.BD.Delete(vIn.TxHash, database.UTXOBucket)
			u.BC.BD.Put(vIn.TxHash, u.serialize(newUTXO), database.UTXOBucket)
		}
	}
}

func (u *UTXOHandle) serialize(utxos []*UTXO) []byte {
	var result bytes.Buffer
	encoder := gob.NewEncoder(&result)

	err := encoder.Encode(utxos)
	if err != nil {
		panic(err)
	}
	return result.Bytes()
}

func (u *UTXOHandle) dserialize(d []byte) []*UTXO {
	var model []*UTXO
	decoder := gob.NewDecoder(bytes.NewReader(d))
	err := decoder.Decode(&model)
	if err != nil {
		log.Panic(err)
	}
	return model
}