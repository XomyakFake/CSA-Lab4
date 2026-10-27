package machine

type IOController struct {
	input  []byte // весь ввод задан заранее, вариант stream
	pos    int
	Output []byte
}

func (io *IOController) read() (value uint32, ok bool) {
	if io.pos >= len(io.input) { // ввод кончился, моделирование остановится
		return 0, false
	}
	value = uint32(io.input[io.pos])
	io.pos++
	return value, true
}

func (io *IOController) write(value uint32) {
	io.Output = append(io.Output, byte(value)) // в вывод идёт младший байт слова
}
