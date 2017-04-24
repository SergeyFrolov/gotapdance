package tapdance

import (
	"encoding/binary"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/zmap/zcrypto/x509"
	"io/ioutil"
	"net"
	"os"
	"path"
	"strconv"
	"sync"
	"reflect"
)

type assets struct {
	sync.RWMutex
	once sync.Once
	path string

	config ClientConf

	roots *x509.CertPool

	filenameStationPubkey string
	filenameRoots         string
	filenameClientConf    string
}

var assetsInstance *assets
var assetsOnce sync.Once

func initTLSDecoySpec(ip string, sni string) *TLSDecoySpec {
	ip4 := net.ParseIP(ip)
	var ipUint32 uint32
	if ip4 != nil {
		ipUint32 = binary.BigEndian.Uint32(net.ParseIP(ip).To4())
	} else {
		ipUint32 = 0
	}
	tlsDecoy := TLSDecoySpec{Hostname: &sni, Ipv4Addr: &ipUint32}
	return &tlsDecoy
}

// Path is expected (but doesn't have) to have several files
// 1) "decoys" that has a list in following format:
//       ip1:SNI1
//       ip2:SNI2
// 2) "station_pubkey" contains TapDance station Public Key
// 3) "roots" contains x509 roots
func Assets() *assets {
	var defaultDecoys = []*TLSDecoySpec{
		initTLSDecoySpec("192.122.190.104", "tapdance1.freeaeskey.xyz"),
		initTLSDecoySpec("192.122.190.105", "tapdance2.freeaeskey.xyz"),
		initTLSDecoySpec("192.122.190.106", "tapdance3.freeaeskey.xyz"),
	}

	defaultKey := []byte{81, 88, 104, 190, 127, 69, 171, 111, 49, 10, 254, 212, 178, 41, 183,
		164, 121, 252, 159, 222, 85, 61, 234, 76, 205, 179, 105, 171, 24, 153, 231, 12}

	defualtKeyType := KeyType_AES_GCM_128
	defaultPubKey := PubKey{Key: defaultKey, Type: &defualtKeyType}
	defaultGeneration := uint32(0)
	defaultDecoyList := DecoyList{TlsDecoys: defaultDecoys}
	defaultClientConf := ClientConf{DecoyList: &defaultDecoyList,
		DefaultPubkey: &defaultPubKey,
		Generation:    &defaultGeneration}

	assetsOnce.Do(func() {
		assetsInstance = &assets{
			path:                  "./assets/",
			config:                defaultClientConf,
			filenameRoots:         "roots",
			filenameClientConf:    "ClientConf",
			filenameStationPubkey: "station_pubkey",
		}
		assetsInstance.readConfigs()
	})
	return assetsInstance
}

func (a *assets) GetAssetsDir() string {
	a.RLock()
	defer a.RUnlock()
	return a.path
}

func (a *assets) SetAssetsDir(path string) {
	a.Lock()
	defer a.Unlock()
	a.path = path
	a.readConfigs()
	return
}

func (a *assets) readConfigs() {
	readRoots := func(filename string) error {
		rootCerts, err := ioutil.ReadFile(filename)
		if err != nil {
			return err
		}
		roots := x509.NewCertPool()
		ok := roots.AppendCertsFromPEM(rootCerts)
		if !ok {
			return errors.New("Failed to parse root certificates")
		} else {
			a.roots = roots
		}
		return nil
	}

	readClientConf := func(filename string) error {
		buf, err := ioutil.ReadFile(filename)
		if err != nil {
			return err
		}
		clientConf := ClientConf{}
		err = proto.Unmarshal(buf, &clientConf)
		if err != nil {
			return err
		}
		a.config = clientConf
		return nil
	}

	readPubkey := func(filename string) error {
		staionPubkey, err := ioutil.ReadFile(filename)
		if err != nil {
			return err
		}
		if len(staionPubkey) != 32 {
			return errors.New("Unexpected keyfile length! Expected: 32. Got: " +
				strconv.Itoa(len(staionPubkey)))
		}
		copy(a.config.DefaultPubkey.Key[:], staionPubkey[0:32])
		return nil
	}

	var err error
	Logger.Infoln("Assets: reading from folder " + a.path)

	rootsFilename := path.Join(a.path, a.filenameRoots)
	err = readRoots(rootsFilename)
	if err != nil {
		Logger.Warningln("Failed to read root ca file: " + err.Error())
	} else {
		Logger.Infoln("X.509 root CAs succesfully read from " + rootsFilename)
	}

	clientConfFilename := path.Join(a.path, a.filenameClientConf)
	err = readClientConf(clientConfFilename)
	if err != nil {
		Logger.Warningln("Failed to read ClientConf file: " + err.Error())
	} else {
		Logger.Infoln("Client config succesfully read from " + clientConfFilename)
	}

	pubkeyFilename := path.Join(a.path, a.filenameStationPubkey)
	err = readPubkey(pubkeyFilename)
	if err != nil {
		Logger.Warningln("Failed to read pubkey file: " + err.Error())
	} else {
		Logger.Infoln("Pubkey succesfully read from " + pubkeyFilename)
	}
}

// gets randomDecoyAddress. sni stands for subject name indication.
// addr is in format ipv4:port
func (a *assets) GetDecoy() (decoy TLSDecoySpec) {
	a.RLock()
	defer a.RUnlock()

	decoys := a.config.DecoyList.TlsDecoys
	if len(decoys) == 0 {
		return TLSDecoySpec{}
	}
	decoyIndex := getRandInt(0, len(decoys)-1)
	decoy = *decoys[decoyIndex]
	return
}

func (a *assets) GetRoots() *x509.CertPool {
	a.RLock()
	defer a.RUnlock()

	return a.roots
}

func (a *assets) GetPubkey() *[32]byte {
	a.RLock()
	defer a.RUnlock()

	var pKey [32]byte
	copy(pKey[:], a.config.DefaultPubkey.Key[:])
	return &pKey
}

func (a *assets) GetGeneration() uint32 {
	a.RLock()
	defer a.RUnlock()

	return a.config.GetGeneration()
}

func (a *assets) SetGeneration(gen uint32) (err error) {
	a.Lock()
	defer a.Unlock()

	copyGen := gen
	a.config.Generation = &copyGen
	err = a.saveClientConf()
	return
}

// Set Public key in persistent way (e.g. store to disk)
func (a *assets) SetPubkey(pubkey PubKey) (err error) {
	a.Lock()
	defer a.Unlock()

	copyPubkey := pubkey
	a.config.DefaultPubkey = &copyPubkey
	err = a.saveClientConf()
	return
}

func (a *assets) SetClientConf(conf *ClientConf) (err error) {
	a.Lock()
	defer a.Unlock()

	a.config = *conf
	err = a.saveClientConf()
	return
}

// Set decoys in persistent way (e.g. store to disk)
func (a *assets) SetDecoys(decoys []*TLSDecoySpec) (err error) {
	a.Lock()
	defer a.Unlock()

	a.config.DecoyList.TlsDecoys = decoys
	err = a.saveClientConf()
	return
}

func (a *assets) IsDecoyInList(ip string, sni string) bool {
	decoy := initTLSDecoySpec(ip, sni)
	for _, d := range a.config.GetDecoyList().GetTlsDecoys() {
		if reflect.DeepEqual(&d, &decoy) {
			return true;
		}
	}
	return false;
}

func (a *assets) saveClientConf() error {
	buf, err := proto.Marshal(&a.config)
	if err != nil {
		return err
	}
	filename := path.Join(a.path, a.filenameClientConf)
	tmpFilename := path.Join(a.path, "."+a.filenameClientConf+".tmp")
	err = ioutil.WriteFile(tmpFilename, buf[:], 0644)
	if err != nil {
		return err
	}

	return os.Rename(tmpFilename, filename)
}
