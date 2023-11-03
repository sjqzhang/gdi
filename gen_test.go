package gdi

import (
	"fmt"
	"testing"
)

func Test_parseMiddlewareAnnotations(t *testing.T) {

	txt:=`xxx,name {tt=10,xx="sdfdf"},xxx,cache   {name=dfd,    adf="ad  f"  }  ,xxxa,xx(asd="sd fd"")`

	mids:=parseMiddlewareAnnotations(txt)

	for _,v:=range mids{
		fmt.Println(v.Name)
		v.Params.Range(func(key, value interface{}) bool {
			t.Log( fmt.Sprintf(`"%s" "%v" "%v"` , v.Name,key,value))
			return true
		})

	}

	if len(mids)!=6{
		t.Error("error")
	}


}
