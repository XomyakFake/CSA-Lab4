package machine

const (
	cacheLines   = 16 // прямое отображение, адрес режем на тег 31..8, строку 7..4, слово 3..2
	wordsPerLine = 4

	missWait      = 9  // промах это 10 тактов, то есть первый такт и ещё 9 ожидания
	missDirtyWait = 19 // с вытеснением изменённой строки 20 тактов
)

type cacheLine struct {
	valid bool
	dirty bool // изменена, но ещё не записана в память
	tag   uint32
	words [wordsPerLine]uint32
}

type Cache struct {
	mem     []uint32 // память за кешем, по словам
	enabled bool     // false для --no-cache, тогда любое обращение идёт в память за 10 тактов
	lines   [cacheLines]cacheLine

	busy bool // идёт обращение к памяти (промах или режим без кеша)
	wait int  // сколько тактов ожидания осталось
}

func newCache(mem []uint32, enabled bool) *Cache {
	return &Cache{mem: mem, enabled: enabled}
}

func (c *Cache) Access(addr uint32, write bool, wdata uint32) (ready bool, rdata uint32, event string) {
	// вызывается каждый такт, пока не вернёт ready, а процессор всё это время повторяет микрокоманду
	line := &c.lines[addr>>4&0xF]
	tag := addr >> 8
	switch {
	case c.busy: // обращение к памяти уже идёт, считаем такты
		c.wait--
		if c.wait > 0 {
			return false, 0, "wait"
		}
		c.busy = false
		event = "ready"
		if c.enabled {
			c.load(line, addr)
			event = "loaded"
		}
	case !c.enabled:
		c.busy, c.wait = true, missWait
		return false, 0, "memory access"
	case line.valid && line.tag == tag:
		event = "hit"
	case line.valid && line.dirty:
		c.busy, c.wait = true, missDirtyWait
		return false, 0, "miss, write back dirty line"
	default:
		c.busy, c.wait = true, missWait
		return false, 0, "miss"
	}

	// обращение завершилось в этом такте
	if !c.enabled {
		if write {
			c.mem[addr/4] = wdata
		}
		return true, c.mem[addr/4], event
	}
	word := addr >> 2 & 3
	if write {
		line.words[word] = wdata
		line.dirty = true // write-back, в память запишем только при вытеснении
	}
	return true, line.words[word], event
}

func (c *Cache) load(line *cacheLine, addr uint32) {
	index := addr >> 4 & 0xF
	if line.valid && line.dirty { // сначала сбрасываем старую изменённую строку в память
		old := (line.tag<<8 | index<<4) / 4
		copy(c.mem[old:old+wordsPerLine], line.words[:])
	}
	base := (addr &^ 0xF) / 4
	copy(line.words[:], c.mem[base:base+wordsPerLine])
	line.valid, line.dirty, line.tag = true, false, addr>>8
}
