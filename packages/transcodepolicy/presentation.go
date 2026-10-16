package transcodepolicy

import (
	"strconv"
	"strings"
)

func PresentationFilterGraph(options Settings, widths []string) string {
	if len(widths) == 0 || options.Accelerator != "none" && options.Accelerator != "" || options.OutputHDR != "" || !options.ToneMap && !options.Deinterlace {
		return ""
	}
	top := widths[len(widths)-1]
	shared := []string{}
	if options.Deinterlace {
		shared = append(shared, "bwdif=mode=send_frame:parity=auto:deint=interlaced")
	}
	shared = append(shared, "scale=w="+top+":h='trunc(ow/dar/2)*2'", "setsar=1")
	if options.ToneMap {
		shared = append(shared, softwareToneMapFilters()...)
	}
	shared = append(shared, "format="+PixelFormat(options))
	var graph, branches strings.Builder
	graph.WriteString("[0:v:0]" + strings.Join(shared, ",") + ",split=" + strconv.Itoa(len(widths)))
	for index, width := range widths {
		label := strconv.Itoa(index)
		graph.WriteString("[t" + label + "]")
		rung := "null"
		if width != top {
			rung = "scale=w=" + width + ":h='trunc(ow/dar/2)*2',setsar=1"
		}
		branches.WriteString(";[t" + label + "]" + rung + "[v" + label + "]")
	}
	return graph.String() + branches.String()
}
