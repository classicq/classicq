package engine

import (
	"bufio"
	"fmt"
	"math"
	"os"
)

// State dump: with -dump <file> the client runs a demo on a fixed clock and
// writes its state as text, one record per line, floats as the hex of their
// IEEE bits. Two dumps of the same demo diff directly.

var dump_file *bufio.Writer
var dump_osfile *os.File
var dump_fps = 77
var dump_seed = 1

func Dump_Init() {
	if i := COM_CheckParm("-dumpfps"); i != 0 && i+1 < com_argc {
		if v := Q_atoi(com_argv[i+1]); v > 0 {
			dump_fps = v
		}
	}
	if i := COM_CheckParm("-dumpseed"); i != 0 && i+1 < com_argc {
		dump_seed = Q_atoi(com_argv[i+1])
	}

	i := COM_CheckParm("-dump")
	if i == 0 || i+1 >= com_argc {
		return
	}
	f, err := os.Create(com_argv[i+1])
	if err != nil {
		Sys_Error("Dump_Init: can't create %s", com_argv[i+1])
	}
	dump_osfile = f
	dump_file = bufio.NewWriterSize(f, 1<<20)
}

func Dump_Active() bool {
	return dump_file != nil
}

func Dump_FrameTime() float64 {
	return 1.0 / float64(dump_fps)
}

func Dump_End() {
	if dump_file == nil {
		return
	}
	dump_file.Flush()
	dump_osfile.Close()
	dump_file = nil
	dump_osfile = nil
}

func hf(f float32) string { return fmt.Sprintf("%08x", math.Float32bits(f)) }
func hd(d float64) string { return fmt.Sprintf("%016x", math.Float64bits(d)) }
func hv(v vec3_t) string  { return hf(v[0]) + " " + hf(v[1]) + " " + hf(v[2]) }

func Dump_Header(demo string) {
	if dump_file == nil {
		return
	}
	fmt.Fprintf(dump_file, "H %s fps=%d seed=%d\n", demo, dump_fps, dump_seed)
}

// start of CL_ParseServerMessage
func Dump_Message() {
	if dump_file == nil {
		return
	}
	fmt.Fprintf(dump_file, "M %d %d\n", cls.netchan.incoming_sequence, cl_net_message.cursize)
}

// every svc_* taken out of the message
func Dump_Svc(cmd int) {
	if dump_file == nil {
		return
	}
	fmt.Fprintf(dump_file, "m %d\n", cmd)
}

func dump_modelIndex(m *model_t) int {
	if m == nil {
		return -1
	}
	for i := range cl.model_precache {
		if cl.model_precache[i] == m {
			return i
		}
	}
	return -2
}

func dump_visents(name string, list *visentlist_t) {
	fmt.Fprintf(dump_file, "R %s %d\n", name, list.count)
	for i := 0; i < list.count; i++ {
		e := &list.list[i]
		fmt.Fprintf(dump_file, "r %d %d %d %s %d %s %s\n",
			dump_modelIndex(e.model), e.frame, e.oldframe, hf(e.framelerp), e.skinnum, hv(e.origin), hv(e.angles))
	}
}

// after CL_EmitEntities in CL_Frame, while the client is active
func Dump_Frame() {
	if dump_file == nil || cls.state != ca_active {
		return
	}
	w := dump_file

	fmt.Fprintf(w, "F %d rt=%s dt=%s ct=%s pc=%d vs=%d tr=%d ac=%d\n",
		cls.framecount, hd(cls.realtime), hd(cls.demotime), hd(cl.time), cl.parsecount, cl.validsequence, spec_track, autocam)
	fmt.Fprintf(w, "V %s %s %s %s\n", hv(cl.simorg), hv(cl.simvel), hv(cl.simangles), hv(cl.viewangles))

	fmt.Fprintf(w, "S")
	for i := 0; i < MAX_CL_STATS; i++ {
		fmt.Fprintf(w, " %d", cl.stats[i])
	}
	fmt.Fprintf(w, "\n")

	pe := &cl.frames[cl.validsequence&UPDATE_MASK].packet_entities
	fmt.Fprintf(w, "E %d\n", pe.num_entities)
	for i := 0; i < pe.num_entities; i++ {
		s := &pe.entities[i]
		fmt.Fprintf(w, "e %d %d %d %d %d %d %s %s\n",
			s.number, s.modelindex, s.frame, s.skinnum, s.colormap, s.effects, hv(s.origin), hv(s.angles))
	}

	frame := &cl.frames[cl.parsecount&UPDATE_MASK]
	for i := 0; i < MAX_CLIENTS; i++ {
		s := &frame.playerstate[i]
		if s.messagenum != cl.parsecount {
			continue
		}
		fmt.Fprintf(w, "P %d %d %d %d %d %d %d %d %d %s %s %s\n",
			i, s.messagenum, s.modelindex, s.frame, s.skinnum, s.effects, s.flags, s.weaponframe, s.pm_type,
			hv(s.origin), hv(s.velocity), hv(s.viewangles))
	}

	dump_visents("first", &cl_firstpassents)
	dump_visents("vis", &cl_visents)
	dump_visents("alpha", &cl_alphaents)
}

// replaces the wall clock main loop: one frame to start the demo from the
// command line, then fixed steps until it ends
func Dump_Main() {
	dt := Dump_FrameTime()

	Host_Frame(dt)
	for frames := 0; cls.demoplayback && frames < 1000000; frames++ {
		Host_Frame(dt)
	}

	Dump_End()
	Host_Quit()
}
