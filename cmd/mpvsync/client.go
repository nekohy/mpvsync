package main

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"net"
	"time"

	"mpvsync/internal/mpv"
)

// client bridges one mpv instance and the peers. All state is owned by the event loop in run.
type client struct {
	start  time.Time
	player *mpv.Client
	udp    net.PacketConn
	peers  []net.Addr

	pos, posT, speed      float64
	name                  string   // current file name
	size                  int64    // current file size
	playlist              []string // file names in the mpv playlist
	hasPos, sawPath       bool
	paused, buffering     bool
	loaded, restarted     bool
	ready                 bool   // a file is loaded and has settled: user actions count from now on
	hadFile               bool   // a file was ready before: the next one is a switch
	followed              string // file we switched to because a peer did (not our own switch)
	missPause             bool   // a peer lacks the file we switched to: pause once ready
	settleAt, readyAt     float64
	expectPause           *bool // pause change we asked for (not a user action)
	ourSeeks              int   // seeks we issued that have not finished (mpv may merge them)
	seeking               bool  // any seek in flight (ours or the user's)
	seekOpAt, ignoreUntil float64
	nextHB                float64
}

type pktIn struct {
	p    Pkt
	addr net.Addr
}

func run(ipcPath string, peers []net.Addr, port int) error {
	player, err := mpv.Dial(ipcPath)
	if err != nil {
		return fmt.Errorf("mpv ipc: %w", err)
	}
	udp, err := net.ListenPacket("udp4", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}
	c := &client{start: time.Now(), player: player, udp: udp, peers: peers, speed: 1, paused: true}
	for i, p := range []string{"pause", "time-pos", "path", "file-size", "playlist", "speed", "paused-for-cache"} {
		c.cmd("observe_property", i+1, p)
	}

	mpvCh, pktCh := player.Events(), make(chan pktIn, 64)
	go func() {
		buf := make([]byte, 64<<10)
		for {
			n, addr, err := udp.ReadFrom(buf)
			if err != nil {
				return
			}
			var p Pkt
			if json.Unmarshal(buf[:n], &p) == nil {
				pktCh <- pktIn{p, addr}
			}
		}
	}()
	tick := time.NewTicker(50 * time.Millisecond)
	for {
		select {
		case e, ok := <-mpvCh:
			if !ok {
				return nil // the player is gone
			}
			c.onMpv(e)
		case in := <-pktCh:
			c.onPkt(in.p, in.addr)
		case <-tick.C:
			c.tick()
		}
	}
}

func (c *client) now() float64 { return time.Since(c.start).Seconds() }

func (c *client) cmd(args ...any) { c.player.Command(args...) }

func (c *client) osd(format string, a ...any) { c.cmd("show-text", fmt.Sprintf(format, a...), 4000) }

func (c *client) curPos() float64 {
	if !c.hasPos || c.paused || c.buffering {
		return c.pos
	}
	return c.pos + (c.now()-c.posT)*c.speed
}

func (c *client) setPos(p float64) { c.pos, c.posT, c.hasPos = p, c.now(), true }

func (c *client) setPause(v bool) {
	c.setPos(c.curPos())
	c.paused, c.expectPause = v, &v
	c.cmd("set_property", "pause", v)
}

func (c *client) seekTo(p float64) {
	c.ourSeeks++
	c.setPos(p)
	c.cmd("seek", math.Max(p, 0), "absolute")
}

func (c *client) send(p Pkt) {
	b, _ := json.Marshal(p)
	for _, a := range c.peers {
		c.udp.WriteTo(b, a)
	}
}

func (c *client) reply(a net.Addr, p Pkt) {
	b, _ := json.Marshal(p)
	c.udp.WriteTo(b, a)
}

// file identifies the current file: hash of its name and size.
func (c *client) file() string {
	h := fnv.New64a()
	fmt.Fprintf(h, "%s\x00%d", c.name, c.size)
	return fmt.Sprintf("%016x", h.Sum64())
}

func (c *client) state(op, seek bool) Pkt {
	return Pkt{Op: op, Seek: seek, File: c.file(), Pos: c.curPos(), Paused: c.paused}
}

// act: the local user paused / played / seeked.
func (c *client) act(seek bool) {
	c.ignoreUntil = c.now() + 1 // heartbeats already in flight predate this
	c.send(c.state(true, seek))
}

func (c *client) onMpv(e mpv.Event) {
	now := c.now()
	switch e.Event {
	case "property-change":
		switch e.Name {
		case "pause":
			var v bool
			json.Unmarshal(e.Data, &v)
			if c.expectPause != nil && *c.expectPause == v {
				c.expectPause = nil
				return
			}
			c.setPos(c.curPos())
			c.paused = v
			if c.ready {
				c.act(false)
			}
		case "time-pos":
			var v *float64
			json.Unmarshal(e.Data, &v)
			if v != nil {
				c.setPos(*v)
			} else {
				c.hasPos = false
			}
		case "speed":
			c.speed = 1
			json.Unmarshal(e.Data, &c.speed)
		case "paused-for-cache":
			c.buffering = false
			json.Unmarshal(e.Data, &c.buffering)
		case "path":
			var p string
			json.Unmarshal(e.Data, &p)
			c.name = fileName(p)
			if !c.sawPath && p != "" {
				c.onLoaded(true) // attached after the file was loaded
			}
			c.sawPath = true
		case "file-size":
			c.size = 0
			json.Unmarshal(e.Data, &c.size)
		case "playlist":
			var list []struct{ Filename string }
			json.Unmarshal(e.Data, &list)
			c.playlist = c.playlist[:0]
			for _, x := range list {
				c.playlist = append(c.playlist, fileName(x.Filename))
			}
		}
	case "file-loaded":
		c.onLoaded(false)
	case "seek":
		c.seeking = true
	case "playback-restart":
		c.seeking = false
		switch {
		case !c.loaded:
		case !c.ready: // initial / resume seeks of a new file are not user actions
			c.restarted, c.settleAt = true, now+1.5
		case c.ourSeeks > 0:
			c.ourSeeks, c.ignoreUntil = 0, now+1 // time-pos needs a moment to catch up
		default:
			c.seekOpAt = now + 0.15 // user seek: report once time-pos has caught up
		}
	case "end-file":
		c.loaded, c.ready, c.seeking, c.ourSeeks = false, false, false, 0
	}
}

func (c *client) onLoaded(alreadyPlaying bool) {
	c.loaded, c.ready, c.restarted, c.missPause = true, false, alreadyPlaying, false
	c.settleAt, c.ourSeeks = c.now()+1.5, 0
	if !alreadyPlaying && c.hadFile && c.name != c.followed { // the user switched files: take the others along
		c.send(Pkt{Load: true, Name: c.name})
	}
	c.followed = ""
}

func (c *client) tick() {
	now := c.now()
	if c.loaded && !c.ready && c.restarted && now >= c.settleAt && c.hasPos {
		c.ready, c.readyAt, c.nextHB, c.hadFile = true, now, 0, true
	}
	if c.missPause && c.ready {
		c.missPause = false
		if !c.paused {
			c.setPause(true)
			c.act(false) // everyone else on this file waits too
		}
	}
	if c.seekOpAt > 0 && now >= c.seekOpAt {
		c.seekOpAt = 0
		if c.ready {
			c.act(true)
		}
	}
	if c.ready && now >= c.nextHB {
		c.nextHB = now + 1
		c.send(c.state(false, false))
	}
}

func (c *client) onPkt(p Pkt, addr net.Addr) {
	from, _, _ := net.SplitHostPort(addr.String())
	switch {
	case p.Load && p.Name != c.name:
		for i, n := range c.playlist { // follow a peer's switch if that file is in our playlist
			if n == p.Name {
				c.followed = p.Name
				c.cmd("playlist-play-index", i)
				c.osd("%s 切到了 %s", from, show(p.Name))
				return
			}
		}
		c.reply(addr, Pkt{Miss: true, Name: p.Name})
		c.osd("%s 切到了 %s，播放列表里没有，请手动打开", from, show(p.Name))
		return
	case p.Load:
		return
	case p.Miss:
		if p.Name == c.name {
			c.missPause = true
			c.osd("%s 的播放列表里没有这一集，已暂停等待", from)
		}
		return
	}
	if !c.ready || p.File != c.file() {
		return
	}
	now, diff := c.now(), p.Pos-c.curPos()
	if p.Op {
		c.seekOpAt, c.ignoreUntil = 0, now+1
		if p.Paused != c.paused {
			c.setPause(p.Paused)
			if p.Paused {
				c.osd("%s 暂停了", from)
			} else {
				c.osd("%s 继续播放", from)
			}
		}
		if p.Seek {
			c.osd("%s 跳转到 %s", from, fmtTime(p.Pos))
		}
		if math.Abs(diff) > 0.3 {
			c.seekTo(p.Pos)
		}
		return
	}
	// heartbeat (only while nothing is in flight here)
	if now < c.ignoreUntil || c.seeking || c.buffering || c.ourSeeks > 0 || c.seekOpAt > 0 {
		return
	}
	switch {
	case p.Paused && now-c.readyAt < 10: // just opened the file while the others are paused: join them
		if c.paused && math.Abs(diff) <= 0.3 {
			return
		}
		if !c.paused {
			c.setPause(true)
		}
		if math.Abs(diff) > 0.3 {
			c.seekTo(p.Pos)
		}
		c.osd("已同步到 %s 的进度 %s（暂停中）", from, fmtTime(p.Pos))
	case diff > 2 && !p.Paused && !c.paused: // behind a playing peer: catch up, nobody is moved backwards
		c.seekTo(p.Pos)
		c.osd("已追上 %s 的进度 %s", from, fmtTime(p.Pos))
	}
}
