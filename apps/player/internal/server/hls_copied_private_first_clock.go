package server

import (
	"context"
	"encoding/binary"
)

// Bounded serialized clock facts remain independent of any presentation map.
// TFDT stays unsigned and is never reinterpreted as a signed clock.
type copiedHLSPrivateFirstClock struct {
	Decode uint64
	Composition int64
	Duration uint32
}

func copiedHLSPrivateAudioClock(ctx context.Context, data []byte, track uint32) (copiedHLSPrivateFirstClock,error) {
	var result copiedHLSPrivateFirstClock
	boxes,err:=copiedHLSPrivateBoxes(data,"styp","moof","mdat","free")
	if err!=nil { return result,err }
	movie,err:=copiedHLSPrivateOne(boxes,"moof")
	if err!=nil { return result,err }
	children,err:=copiedHLSPrivateBoxes(movie.data,"mfhd","traf")
	if err!=nil { return result,err }
	found:=false
	for _,child:=range children {
		if child.kind!="traf" { continue }
		if ctx.Err()!=nil { return result,errCopiedHLSIndex }
		clock,selected,err:=copiedHLSPrivateTrackClock(child.data,track)
		if err!=nil||selected&&found { return result,errCopiedHLSIndex }
		if selected { result,found=clock,true }
	}
	if !found||ctx.Err()!=nil { return result,errCopiedHLSIndex }
	return result,nil
}

func copiedHLSPrivateTrackClock(data []byte, track uint32) (copiedHLSPrivateFirstClock,bool,error) {
	var result copiedHLSPrivateFirstClock
	parts,err:=copiedHLSPrivateBoxes(data,"tfhd","tfdt","trun")
	if err!=nil { return result,false,err }
	header,err:=copiedHLSPrivateOne(parts,"tfhd")
	if err!=nil { return result,false,err }
	defaults,err:=copiedHLSPrivateFragmentDefaults(header.data)
	if err!=nil { return result,false,err }
	if defaults.id!=track { return result,false,nil }
	result,err=copiedHLSPrivateTrackFirstClock(parts,defaults)
	return result,true,err
}

func copiedHLSPrivateTrackFirstClock(parts []copiedHLSPrivateBox, defaults copiedHLSPrivateDefaults) (copiedHLSPrivateFirstClock,error) {
	var result copiedHLSPrivateFirstClock
	decode,err:=copiedHLSPrivateOne(parts,"tfdt")
	if err!=nil||!copiedHLSPrivateDecodeShape(decode.data) { return result,errCopiedHLSIndex }
	if decode.data[0]==0 { result.Decode=uint64(binary.BigEndian.Uint32(decode.data[4:])) } else { result.Decode=binary.BigEndian.Uint64(decode.data[4:]) }
	run,err:=copiedHLSPrivateOne(parts,"trun")
	if err!=nil { return result,err }
	result.Duration,result.Composition,err=copiedHLSPrivateFirstSampleClock(run.data,defaults)
	return result,err
}

func copiedHLSPrivateFirstSampleClock(data []byte, defaults copiedHLSPrivateDefaults) (uint32,int64,error) {
	flags,_,position,err:=copiedHLSPrivateRunHeader(data)
	if err!=nil { return 0,0,err }
	if _,err=copiedHLSPrivateUint32(data,&position);err!=nil { return 0,0,err }
	if flags&4!=0 {
		if _,err=copiedHLSPrivateUint32(data,&position);err!=nil { return 0,0,err }
	}
	values:=[4]uint32{defaults.duration,defaults.size,defaults.flags,0}
	for index:=range values {
		if flags&(uint32(0x100)<<index)==0 { continue }
		values[index],err=copiedHLSPrivateUint32(data,&position)
		if err!=nil { return 0,0,err }
	}
	composition:=int64(values[3])
	if data[0]==1&&values[3]&(1<<31)!=0 { composition-=1<<32 }
	if values[0]==0||values[1]==0 { return 0,0,errCopiedHLSIndex }
	return values[0],composition,nil
}
