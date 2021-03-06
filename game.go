package main // unit: Game

import (
	"os"
	"path/filepath"
	"time"
)

// interface uses: GameVars, TxtWind

const (
	PROMPT_NUMERIC  = 0
	PROMPT_ALPHANUM = 1
	PROMPT_ANY      = 2
)
const LineChars string = "\xf9\xd0Һ\xb5\xbc\xbb\xb9\xc6\xc8\xc9\xcc\xcd\xca\xcb\xce"

var (
	ProgressAnimColors  [8]byte   = [8]byte{0x14, 0x1C, 0x15, 0x1D, 0x16, 0x1E, 0x17, 0x1F}
	ProgressAnimStrings [8]string = [8]string{"....|", "...*/", "..*.-", ".*..\\", "*...|", "..../", "....-", "....\\"}
	ColorNames          [7]string = [7]string{"Blue", "Green", "Cyan", "Red", "Purple", "Yellow", "White"}
	DiagonalDeltaX      [8]int16  = [8]int16{-1, 0, 1, 1, 1, 0, -1, -1}
	DiagonalDeltaY      [8]int16  = [8]int16{1, 1, 1, 0, -1, -1, -1, 0}
	NeighborDeltaX      [4]int16  = [4]int16{0, 0, -1, 1}
	NeighborDeltaY      [4]int16  = [4]int16{-1, 1, 0, 0}
	TileBorder          TTile     = TTile{Element: E_NORMAL, Color: 0x0E}
	TileBoardEdge       TTile     = TTile{Element: E_BOARD_EDGE, Color: 0x00}
	StatTemplateDefault TStat     = TStat{X: 0, Y: 0, StepX: 0, StepY: 0, Cycle: 0, P1: 0, P2: 0, P3: 0, Follower: -1, Leader: -1}
)

// implementation uses: Dos, Crt, Video, Sounds, Input, Elements, Editor, Oop

func SidebarClearLine(y int16) {
	VideoWriteText(60, y, 0x11, "\xb3                   ")
}

func SidebarClear() {
	var i int16
	for i = 3; i <= 24; i++ {
		SidebarClearLine(i)
	}
}

func GenerateTransitionTable() {
	var (
		ix, iy int16
		t      TCoord
	)
	TransitionTableSize = 0
	for iy = 1; iy <= BOARD_HEIGHT; iy++ {
		for ix = 1; ix <= BOARD_WIDTH; ix++ {
			TransitionTableSize++
			TransitionTable[TransitionTableSize-1].X = ix
			TransitionTable[TransitionTableSize-1].Y = iy
		}
	}
	for ix = 1; ix <= TransitionTableSize; ix++ {
		iy = Random(TransitionTableSize) + 1
		t = TransitionTable[iy-1]
		TransitionTable[iy-1] = TransitionTable[ix-1]
		TransitionTable[ix-1] = t
	}
}

func BoardClose() {
	var (
		ix, iy int16
		ptr    []byte
		rle    TRleTile
	)
	ptr = IoTmpBuf[:]
	StoreString(ptr[:SizeOfBoardName], Board.Name)
	ptr = ptr[SizeOfBoardName:]

	ix = 1
	iy = 1
	rle.Count = 1
	rle.Tile = Board.Tiles[ix][iy]
	for {
		ix++
		if ix > BOARD_WIDTH {
			ix = 1
			iy++
		}
		if Board.Tiles[ix][iy].Color == rle.Tile.Color && Board.Tiles[ix][iy].Element == rle.Tile.Element && rle.Count < 255 && iy <= BOARD_HEIGHT {
			rle.Count++
		} else {
			StoreRleTile(ptr[:SizeOfRleTile], rle)
			ptr = ptr[SizeOfRleTile:]
			rle.Tile = Board.Tiles[ix][iy]
			rle.Count = 1
		}
		if iy > BOARD_HEIGHT {
			break
		}
	}

	StoreBoardInfo(ptr[:SizeOfBoardInfo], &Board.Info)
	ptr = ptr[SizeOfBoardInfo:]

	StoreInt16(ptr[:2], Board.StatCount)
	ptr = ptr[2:]

	for ix = 0; ix <= Board.StatCount; ix++ {
		stat := &Board.Stats[ix]
		if stat.DataLen > 0 {
			for iy = 1; iy <= ix-1; iy++ {
				if Board.Stats[iy].Data == stat.Data {
					stat.DataLen = -iy
				}
			}
		}
		StoreStat(ptr[:SizeOfStat], &Board.Stats[ix])
		ptr = ptr[SizeOfStat:]

		if stat.DataLen > 0 {
			copy(ptr[:stat.DataLen], stat.Data)
			ptr = ptr[stat.DataLen:]
		}
	}

	boardData := IoTmpBuf[:len(IoTmpBuf)-len(ptr)]
	World.BoardLen[World.Info.CurrentBoard] = int16(len(boardData))
	World.BoardData[World.Info.CurrentBoard] = make([]byte, len(boardData))
	copy(World.BoardData[World.Info.CurrentBoard], boardData)
}

func BoardOpen(boardId int16) {
	var (
		ptr    []byte
		ix, iy int16
		rle    TRleTile
	)
	if boardId > World.BoardCount {
		boardId = World.Info.CurrentBoard
	}
	ptr = World.BoardData[boardId]
	Board.Name = LoadString(ptr[:SizeOfBoardName])
	ptr = ptr[SizeOfBoardName:]

	ix = 1
	iy = 1
	rle.Count = 0
	for {
		if rle.Count <= 0 {
			rle = LoadRleTile(ptr[:SizeOfRleTile])
			ptr = ptr[SizeOfRleTile:]
		}
		Board.Tiles[ix][iy] = rle.Tile
		ix++
		if ix > BOARD_WIDTH {
			ix = 1
			iy++
		}
		rle.Count--
		if iy > BOARD_HEIGHT {
			break
		}
	}

	LoadBoardInfo(ptr[:SizeOfBoardInfo], &Board.Info)
	ptr = ptr[SizeOfBoardInfo:]

	Board.StatCount = LoadInt16(ptr[:2])
	ptr = ptr[2:]

	for ix = 0; ix <= Board.StatCount; ix++ {
		stat := &Board.Stats[ix]
		LoadStat(ptr[:SizeOfStat], &Board.Stats[ix])
		ptr = ptr[SizeOfStat:]

		if stat.DataLen > 0 {
			stat.Data = string(ptr[:stat.DataLen])
			ptr = ptr[stat.DataLen:]
		} else if stat.DataLen < 0 {
			stat.Data = Board.Stats[-stat.DataLen].Data
			stat.DataLen = Board.Stats[-stat.DataLen].DataLen
		}

	}

	World.Info.CurrentBoard = boardId
}

func BoardChange(boardId int16) {
	Board.Tiles[Board.Stats[0].X][Board.Stats[0].Y].Element = E_PLAYER
	Board.Tiles[Board.Stats[0].X][Board.Stats[0].Y].Color = ElementDefs[E_PLAYER].Color
	BoardClose()
	BoardOpen(boardId)
}

func BoardCreate() {
	var ix, iy, i int16
	Board.Name = ""
	Board.Info.Message = ""
	Board.Info.MaxShots = 255
	Board.Info.IsDark = false
	Board.Info.ReenterWhenZapped = false
	Board.Info.TimeLimitSec = 0
	for i = 0; i <= 3; i++ {
		Board.Info.NeighborBoards[i] = 0
	}
	for ix = 0; ix <= BOARD_WIDTH+1; ix++ {
		Board.Tiles[ix][0] = TileBoardEdge
		Board.Tiles[ix][BOARD_HEIGHT+1] = TileBoardEdge
	}
	for iy = 0; iy <= BOARD_HEIGHT+1; iy++ {
		Board.Tiles[0][iy] = TileBoardEdge
		Board.Tiles[BOARD_WIDTH+1][iy] = TileBoardEdge
	}
	for ix = 1; ix <= BOARD_WIDTH; ix++ {
		for iy = 1; iy <= BOARD_HEIGHT; iy++ {
			Board.Tiles[ix][iy].Element = E_EMPTY
			Board.Tiles[ix][iy].Color = 0
		}
	}
	for ix = 1; ix <= BOARD_WIDTH; ix++ {
		Board.Tiles[ix][1] = TileBorder
		Board.Tiles[ix][BOARD_HEIGHT] = TileBorder
	}
	for iy = 1; iy <= BOARD_HEIGHT; iy++ {
		Board.Tiles[1][iy] = TileBorder
		Board.Tiles[BOARD_WIDTH][iy] = TileBorder
	}
	Board.Tiles[BOARD_WIDTH/2][BOARD_HEIGHT/2].Element = E_PLAYER
	Board.Tiles[BOARD_WIDTH/2][BOARD_HEIGHT/2].Color = ElementDefs[E_PLAYER].Color
	Board.StatCount = 0
	Board.Stats[0].X = BOARD_WIDTH / 2
	Board.Stats[0].Y = BOARD_HEIGHT / 2
	Board.Stats[0].Cycle = 1
	Board.Stats[0].Under.Element = E_EMPTY
	Board.Stats[0].Under.Color = 0
	Board.Stats[0].Data = ""
	Board.Stats[0].DataLen = 0
}

func WorldCreate() {
	var i int16
	InitElementsGame()
	World.BoardCount = 0
	World.BoardLen[0] = 0
	InitEditorStatSettings()
	ResetMessageNotShownFlags()
	BoardCreate()
	World.Info.IsSave = false
	World.Info.CurrentBoard = 0
	World.Info.Ammo = 0
	World.Info.Gems = 0
	World.Info.Health = 100
	World.Info.EnergizerTicks = 0
	World.Info.Torches = 0
	World.Info.TorchTicks = 0
	World.Info.Score = 0
	World.Info.BoardTimeSec = 0
	World.Info.BoardTimeHsec = 0
	for i = 1; i <= 7; i++ {
		World.Info.Keys[i-1] = false
	}
	for i = 1; i <= 10; i++ {
		World.Info.Flags[i-1] = ""
	}
	BoardChange(0)
	Board.Name = "Title screen"
	LoadedGameFileName = ""
	World.Info.Name = ""
}

func TransitionDrawToFill(chr byte, color int16) {
	var i int16
	for i = 1; i <= TransitionTableSize; i++ {
		VideoWriteText(TransitionTable[i-1].X-1, TransitionTable[i-1].Y-1, byte(color), string([]byte{chr}))
	}
}

func TileToColorAndChar(x, y int16) (color, char byte) {
	var ch byte
	tile := &Board.Tiles[x][y]
	if !Board.Info.IsDark || ElementDefs[Board.Tiles[x][y].Element].VisibleInDark || World.Info.TorchTicks > 0 && Sqr(int16(Board.Stats[0].X)-x)+Sqr(int16(Board.Stats[0].Y)-y)*2 < TORCH_DIST_SQR || ForceDarknessOff {
		if tile.Element == E_EMPTY {
			return 0x0F, ' '
		} else if ElementDefs[tile.Element].HasDrawProc {
			ElementDefs[tile.Element].DrawProc(x, y, &ch)
			return tile.Color, ch
		} else if tile.Element < E_TEXT_MIN {
			return tile.Color, ElementDefs[tile.Element].Character
		} else {
			if tile.Element == E_TEXT_WHITE {
				return 0x0F, Board.Tiles[x][y].Color
			} else {
				return byte((int16(tile.Element-E_TEXT_MIN)+1)*16 + 0x0F), Board.Tiles[x][y].Color
			}
		}
	} else {
		return 0x07, '\xb0'
	}
}

func BoardDrawTile(x, y int16) {
	color, char := TileToColorAndChar(x, y)
	VideoWriteText(x-1, y-1, color, string([]byte{char}))
}

func BoardDrawBorder() {
	var ix, iy int16
	for ix = 1; ix <= BOARD_WIDTH; ix++ {
		BoardDrawTile(ix, 1)
		BoardDrawTile(ix, BOARD_HEIGHT)
	}
	for iy = 1; iy <= BOARD_HEIGHT; iy++ {
		BoardDrawTile(1, iy)
		BoardDrawTile(BOARD_WIDTH, iy)
	}
}

func TransitionDrawToBoard() {
	var i int16
	BoardDrawBorder()
	for i = 1; i <= TransitionTableSize; i++ {
		table := &TransitionTable[i-1]
		BoardDrawTile(table.X, table.Y)
	}
}

func SidebarPromptCharacter(editable bool, x, y int16, prompt string, value *byte) {
	var i, newValue int16
	SidebarClearLine(y)
	VideoWriteText(x, y, byte(BoolToInt(editable)+0x1E), prompt)
	SidebarClearLine(y + 1)
	VideoWriteText(x+5, y+1, 0x9F, "\x1f")
	SidebarClearLine(y + 2)
	for {
		for i = int16(*value) - 4; i <= int16(*value)+4; i++ {
			VideoWriteText(x+i-int16(*value)+5, y+2, 0x1E, Chr(byte((i+0x100)%0x100)))
		}
		if editable {
			InputReadWaitKey()
			if InputKeyPressed == KEY_TAB {
				InputDeltaX = 9
			}
			newValue = int16(*value) + InputDeltaX
			if int16(*value) != newValue {
				*value = byte((newValue + 0x100) % 0x100)
				SidebarClearLine(y + 2)
			}
		}
		if InputKeyPressed == KEY_ENTER || InputKeyPressed == KEY_ESCAPE || !editable || InputShiftPressed {
			break
		}
	}
	VideoWriteText(x+5, y+1, 0x1F, "\x1f")
}

func SidebarPromptSlider(editable bool, x, y int16, prompt string, value *byte) {
	var (
		newValue           int16
		startChar, endChar byte
	)
	if prompt[Length(prompt)-3] == ';' {
		startChar = prompt[Length(prompt)-2]
		endChar = prompt[Length(prompt)-1]
		prompt = Copy(prompt, 1, Length(prompt)-3)
	} else {
		startChar = '1'
		endChar = '9'
	}
	SidebarClearLine(y)
	VideoWriteText(x, y, byte(BoolToInt(editable)+0x1E), prompt)
	SidebarClearLine(y + 1)
	SidebarClearLine(y + 2)
	VideoWriteText(x, y+2, 0x1E, string([]byte{startChar})+"....:...."+string([]byte{endChar}))
	for {
		if editable {
			VideoWriteText(x+int16(*value)+1, y+1, 0x9F, "\x1f")
			InputReadWaitKey()
			if InputKeyPressed >= '1' && InputKeyPressed <= '9' {
				*value = Ord(InputKeyPressed) - 49
				SidebarClearLine(y + 1)
			} else {
				newValue = int16(*value) + InputDeltaX
				if int16(*value) != newValue && newValue >= 0 && newValue <= 8 {
					*value = byte(newValue)
					SidebarClearLine(y + 1)
				}
			}
		}
		if InputKeyPressed == KEY_ENTER || InputKeyPressed == KEY_ESCAPE || !editable || InputShiftPressed {
			break
		}
	}
	VideoWriteText(x+int16(*value)+1, y+1, 0x1F, "\x1f")
}

func SidebarPromptChoice(editable bool, y int16, prompt, choiceStr string, result *byte) {
	var (
		i, j, choiceCount int16
		newResult         int16
	)
	SidebarClearLine(y)
	SidebarClearLine(y + 1)
	SidebarClearLine(y + 2)
	VideoWriteText(63, y, byte(BoolToInt(editable)+0x1E), prompt)
	VideoWriteText(63, y+2, 0x1E, choiceStr)
	choiceCount = 1
	for i = 1; i <= Length(choiceStr); i++ {
		if choiceStr[i-1] == ' ' {
			choiceCount++
		}
	}
	for {
		j = 0
		i = 1
		for j < int16(*result) && i < Length(choiceStr) {
			if choiceStr[i-1] == ' ' {
				j++
			}
			i++
		}
		if editable {
			VideoWriteText(62+i, y+1, 0x9F, "\x1f")
			InputReadWaitKey()
			newResult = int16(*result) + InputDeltaX
			if int16(*result) != newResult && newResult >= 0 && newResult <= choiceCount-1 {
				*result = byte(newResult)
				SidebarClearLine(y + 1)
			}
		}
		if InputKeyPressed == KEY_ENTER || InputKeyPressed == KEY_ESCAPE || !editable || InputShiftPressed {
			break
		}
	}
	VideoWriteText(62+i, y+1, 0x1F, "\x1f")
}

func SidebarPromptDirection(editable bool, y int16, prompt string, deltaX, deltaY *int16) {
	var choice byte
	if *deltaY == -1 {
		choice = 0
	} else if *deltaY == 1 {
		choice = 1
	} else if *deltaX == -1 {
		choice = 2
	} else {
		choice = 3
	}

	SidebarPromptChoice(editable, y, prompt, "\x18 \x19 \x1b \x1a", &choice)
	*deltaX = NeighborDeltaX[choice]
	*deltaY = NeighborDeltaY[choice]
}

func PromptString(x, y, arrowColor, color, width int16, mode byte, buffer *string) {
	var (
		i             int16
		oldBuffer     string
		firstKeyPress bool
	)
	oldBuffer = *buffer
	firstKeyPress = true
	for {
		for i = 0; i <= width-1; i++ {
			VideoWriteText(x+i, y, byte(color), " ")
			VideoWriteText(x+i, y-1, byte(arrowColor), " ")
		}
		VideoWriteText(x+width, y-1, byte(arrowColor), " ")
		VideoWriteText(x+Length(*buffer), y-1, byte(arrowColor/0x10*16+0x0F), "\x1f")
		VideoWriteText(x, y, byte(color), *buffer)
		InputReadWaitKey()
		if Length(*buffer) < width && InputKeyPressed >= ' ' && InputKeyPressed < '\x80' {
			if firstKeyPress {
				*buffer = ""
			}
			switch mode {
			case PROMPT_NUMERIC:
				if InputKeyPressed >= '0' && InputKeyPressed <= '9' {
					*buffer += string([]byte{InputKeyPressed})
				}
			case PROMPT_ANY:
				*buffer += string([]byte{InputKeyPressed})
			case PROMPT_ALPHANUM:
				if UpCase(InputKeyPressed) >= 'A' && UpCase(InputKeyPressed) <= 'Z' || InputKeyPressed >= '0' && InputKeyPressed <= '9' || InputKeyPressed == '-' {
					*buffer += string([]byte{UpCase(InputKeyPressed)})
				}
			}
		} else if InputKeyPressed == KEY_LEFT || InputKeyPressed == KEY_BACKSPACE {
			*buffer = Copy(*buffer, 1, Length(*buffer)-1)
		}

		firstKeyPress = false
		if InputKeyPressed == KEY_ENTER || InputKeyPressed == KEY_ESCAPE {
			break
		}
	}
	if InputKeyPressed == KEY_ESCAPE {
		*buffer = oldBuffer
	}
}

func SidebarPromptYesNo(message string, defaultReturn bool) (SidebarPromptYesNo bool) {
	SidebarClearLine(3)
	SidebarClearLine(4)
	SidebarClearLine(5)
	VideoWriteText(63, 5, 0x1F, message)
	VideoWriteText(63+Length(message), 5, 0x9E, "_")
	for {
		InputReadWaitKey()
		if UpCase(InputKeyPressed) == KEY_ESCAPE || UpCase(InputKeyPressed) == 'N' || UpCase(InputKeyPressed) == 'Y' {
			break
		}
	}
	if UpCase(InputKeyPressed) == 'Y' {
		defaultReturn = true
	} else {
		defaultReturn = false
	}
	SidebarClearLine(5)
	SidebarPromptYesNo = defaultReturn
	return
}

func SidebarPromptString(prompt string, extension string, filename *string, promptMode byte) {
	SidebarClearLine(3)
	SidebarClearLine(4)
	SidebarClearLine(5)
	VideoWriteText(75-Length(prompt), 3, 0x1F, prompt)
	VideoWriteText(63, 5, 0x0F, "        "+extension)
	PromptString(63, 5, 0x1E, 0x0F, 8, promptMode, filename)
	SidebarClearLine(3)
	SidebarClearLine(4)
	SidebarClearLine(5)
}

func PauseOnError() {
	SoundQueue(1, SoundParse("s004x114x9"))
	Delay(2000)
}

func DisplayIOError(err error) (DisplayIOError bool) {
	var (
		textWindow TTextWindowState
	)
	if err == nil {
		DisplayIOError = false
		return
	}
	DisplayIOError = true
	textWindow.Title = err.Error()[:40]
	textWindow.Title = "Error: " + textWindow.Title
	TextWindowInitState(&textWindow)
	TextWindowAppend(&textWindow, "OS Error:")
	TextWindowAppend(&textWindow, "")
	TextWindowAppend(&textWindow, "This may be caused by missing")
	TextWindowAppend(&textWindow, "ZZT files or a bad disk.  If")
	TextWindowAppend(&textWindow, "you are trying to save a game,")
	TextWindowAppend(&textWindow, "your disk may be full -- try")
	TextWindowAppend(&textWindow, "using a blank, formatted disk")
	TextWindowAppend(&textWindow, "for saving the game!")
	TextWindowDrawOpen(&textWindow)
	TextWindowSelect(&textWindow, false, false)
	TextWindowDrawClose(&textWindow)
	TextWindowFree(&textWindow)
	return
}

func WorldUnload() {
	BoardClose()
}

func WorldLoad(filename, extension string, titleOnly bool) (WorldLoad bool) {
	var (
		ptr          []byte
		boardId      int16
		loadProgress int16
	)
	SidebarAnimateLoading := func() {
		VideoWriteText(69, 5, ProgressAnimColors[loadProgress], ProgressAnimStrings[loadProgress])
		loadProgress = (loadProgress + 1) % 8
	}

	WorldLoad = false
	loadProgress = 0
	SidebarClearLine(4)
	SidebarClearLine(5)
	SidebarClearLine(5)
	VideoWriteText(62, 5, 0x1F, "Loading.....")

	f, err := os.Open(filename + extension)
	if DisplayIOError(err) {
		return
	}
	defer f.Close()

	WorldUnload()
	_, err = f.Read(IoTmpBuf[:512])
	if DisplayIOError(err) {
		return
	}

	ptr = IoTmpBuf[:]
	World.BoardCount = LoadInt16(ptr[:2])
	ptr = ptr[2:]

	if World.BoardCount < 0 {
		if World.BoardCount != -1 {
			VideoWriteText(63, 5, 0x1E, "You need a newer")
			VideoWriteText(63, 6, 0x1E, " version of ZZT!")
			return
		} else {
			World.BoardCount = LoadInt16(ptr[:2])
			ptr = ptr[2:]
		}
	}

	LoadWorldInfo(ptr[:SizeOfWorldInfo], &World.Info)
	ptr = ptr[SizeOfWorldInfo:]

	if titleOnly {
		World.BoardCount = 0
		World.Info.CurrentBoard = 0
		World.Info.IsSave = true
	}

	for boardId = 0; boardId <= World.BoardCount; boardId++ {
		SidebarAnimateLoading()

		lenBuf := make([]byte, 2)
		_, err = f.Read(lenBuf)
		if DisplayIOError(err) {
			return
		}
		World.BoardLen[boardId] = LoadInt16(lenBuf)

		World.BoardData[boardId] = make([]byte, World.BoardLen[boardId])
		_, err = f.Read(World.BoardData[boardId])
		if DisplayIOError(err) {
			return
		}
	}

	BoardOpen(World.Info.CurrentBoard)
	LoadedGameFileName = filename
	WorldLoad = true
	HighScoresLoad()
	SidebarClearLine(5)

	return
}

func WorldSave(filename, extension string) {
	var (
		i   int16
		ptr []byte
	)

	BoardClose()
	defer func() {
		BoardOpen(World.Info.CurrentBoard)
		SidebarClearLine(5)
	}()

	VideoWriteText(63, 5, 0x1F, "Saving...")

	f, err := os.Create(filename + extension)
	if DisplayIOError(err) {
		return
	}
	defer f.Close()

	ptr = IoTmpBuf[:]
	for i := 0; i < 512; i++ {
		ptr[0] = 0
	}
	StoreInt16(ptr[:2], -1)
	ptr = ptr[2:]
	StoreInt16(ptr[:2], World.BoardCount)
	ptr = ptr[2:]
	StoreWorldInfo(ptr[:SizeOfWorldInfo], &World.Info)
	ptr = ptr[SizeOfWorldInfo:]
	_, err = f.Write(IoTmpBuf[:512])
	if DisplayIOError(err) {
		return
	}

	for i = 0; i <= World.BoardCount; i++ {
		lenBuf := make([]byte, 2)
		StoreInt16(lenBuf, World.BoardLen[i])
		_, err = f.Write(lenBuf)
		if DisplayIOError(err) {
			return
		}

		_, err = f.Write(World.BoardData[i])
		if DisplayIOError(err) {
			return
		}
	}
}

func GameWorldSave(prompt string, filename *string, extension string) {
	var newFilename string
	newFilename = *filename
	SidebarPromptString(prompt, extension, &newFilename, PROMPT_ALPHANUM)
	if InputKeyPressed != KEY_ESCAPE && Length(newFilename) != 0 {
		*filename = newFilename
		if extension == ".ZZT" {
			World.Info.Name = *filename
		}
		WorldSave(*filename, extension)
	}
}

func GameWorldLoad(extension string) (GameWorldLoad bool) {
	var (
		textWindow TTextWindowState
		entryName  string
		i          int16
	)

	TextWindowInitState(&textWindow)
	if extension == ".ZZT" {
		textWindow.Title = "ZZT Worlds"
	} else {
		textWindow.Title = "Saved Games"
	}
	GameWorldLoad = false
	textWindow.Selectable = true

	matches, err := filepath.Glob("*" + extension)
	if err == nil {
		for _, match := range matches {
			entryName = match[:len(match)-4]
			for i = 1; i <= WorldFileDescCount; i++ {
				if entryName == WorldFileDescKeys[i-1] {
					entryName = WorldFileDescValues[i-1]
				}
			}
			TextWindowAppend(&textWindow, entryName)
		}
	}

	TextWindowAppend(&textWindow, "Exit")
	TextWindowDrawOpen(&textWindow)
	TextWindowSelect(&textWindow, false, false)
	TextWindowDrawClose(&textWindow)

	if textWindow.LinePos < textWindow.LineCount && !TextWindowRejected {
		entryName = textWindow.Lines[textWindow.LinePos-1]
		if Pos(' ', entryName) != 0 {
			entryName = Copy(entryName, 1, Pos(' ', entryName)-1)
		}
		GameWorldLoad = WorldLoad(entryName, extension, false)
		TransitionDrawToFill('\xdb', 0x44)
	}

	return
}

func HighScoresAdd(score int16) {
	var (
		textWindow TTextWindowState
		name       string
		i, listPos int16
	)
	listPos = 1
	for listPos <= 30 && score < HighScoreList[listPos-1].Score {
		listPos++
	}
	if listPos <= 30 && score > 0 {
		for i = 29; i >= listPos; i-- {
			HighScoreList[i+1-1] = HighScoreList[i-1]
		}
		HighScoreList[listPos-1].Score = score
		HighScoreList[listPos-1].Name = "-- You! --"
		HighScoresInitTextWindow(&textWindow)
		textWindow.LinePos = listPos
		textWindow.Title = "New high score for " + World.Info.Name
		TextWindowDrawOpen(&textWindow)
		TextWindowDraw(&textWindow, false, false)
		name = ""
		PopupPromptString("Congratulations!  Enter your name:", &name)
		HighScoreList[listPos-1].Name = name
		HighScoresSave()
		TextWindowDrawClose(&textWindow)
		TransitionDrawToBoard()
		TextWindowFree(&textWindow)
	}
}

func HighScoresLoad() {
	f, err := os.Open(World.Info.Name + ".HI")
	if err == nil {
		buf := make([]byte, SizeOfHighScoreList)
		_, err = f.Read(buf)
		if err == nil {
			LoadHighScoreList(buf, HighScoreList[:])
		}
		f.Close()
	}
	if err != nil {
		for i := 0; i < HIGH_SCORE_COUNT; i++ {
			HighScoreList[i].Name = ""
			HighScoreList[i].Score = -1
		}
	}
}

func HighScoresSave() {
	f, err := os.Create(World.Info.Name + ".HI")
	if err != nil {
		DisplayIOError(err)
		return
	}
	buf := make([]byte, SizeOfHighScoreList)
	StoreHighScoreList(buf, HighScoreList[:])
	_, err = f.Write(buf)
	if err != nil {
		DisplayIOError(err)
		return
	}
	f.Close()
}

func HighScoresInitTextWindow(state *TTextWindowState) {
	TextWindowInitState(state)
	TextWindowAppend(state, "Score  Name")
	TextWindowAppend(state, "-----  ----------------------------------")
	for i := 0; i < HIGH_SCORE_COUNT; i++ {
		if Length(HighScoreList[i].Name) != 0 {
			scoreStr := StrWidth(HighScoreList[i].Score, 5)
			TextWindowAppend(state, scoreStr+"  "+HighScoreList[i].Name)
		}
	}
}

func HighScoresDisplay(linePos int16) {
	var state TTextWindowState
	state.LinePos = linePos
	HighScoresInitTextWindow(&state)
	if state.LineCount > 2 {
		state.Title = "High scores for " + World.Info.Name
		TextWindowDrawOpen(&state)
		TextWindowSelect(&state, false, true)
		TextWindowDrawClose(&state)
	}
	TextWindowFree(&state)
}

func CopyStatDataToTextWindow(statId int16, state *TTextWindowState) {
	stat := &Board.Stats[statId]
	TextWindowInitState(state)

	var dataBuf []byte
	for i := 0; i < int(stat.DataLen); i++ {
		dataChr := stat.Data[i]
		if dataChr == KEY_ENTER {
			TextWindowAppend(state, string(dataBuf))
			dataBuf = dataBuf[:0]
		} else {
			dataBuf = append(dataBuf, dataChr)
		}
	}
}

func AddStat(tx, ty int16, element byte, color, tcycle int16, template TStat) {
	if Board.StatCount < MAX_STAT {
		Board.StatCount++
		Board.Stats[Board.StatCount] = template
		stat := &Board.Stats[Board.StatCount]
		stat.X = byte(tx)
		stat.Y = byte(ty)
		stat.Cycle = tcycle
		stat.Under = Board.Tiles[tx][ty]
		stat.DataPos = 0
		if template.Data != "" {
			Board.Stats[Board.StatCount].Data = template.Data
		}
		if ElementDefs[Board.Tiles[tx][ty].Element].PlaceableOnTop {
			Board.Tiles[tx][ty].Color = byte(color&0x0F + int16(Board.Tiles[tx][ty].Color)&0x70)
		} else {
			Board.Tiles[tx][ty].Color = byte(color)
		}
		Board.Tiles[tx][ty].Element = element
		if ty > 0 {
			BoardDrawTile(tx, ty)
		}
	}
}

func RemoveStat(statId int16) {
	var i int16
	stat := &Board.Stats[statId]
	if stat.DataLen != 0 {
		for i = 1; i <= Board.StatCount; i++ {
			if Board.Stats[i].Data == stat.Data && i != statId {
				goto StatDataInUse
			}
		}
		stat.Data = ""
	}
StatDataInUse:
	if statId < CurrentStatTicked {
		CurrentStatTicked--
	}

	Board.Tiles[stat.X][stat.Y] = stat.Under
	if stat.Y > 0 {
		BoardDrawTile(int16(stat.X), int16(stat.Y))
	}
	for i = 1; i <= Board.StatCount; i++ {
		if Board.Stats[i].Follower >= statId {
			if Board.Stats[i].Follower == statId {
				Board.Stats[i].Follower = -1
			} else {
				Board.Stats[i].Follower--
			}
		}
		if Board.Stats[i].Leader >= statId {
			if Board.Stats[i].Leader == statId {
				Board.Stats[i].Leader = -1
			} else {
				Board.Stats[i].Leader--
			}
		}
	}
	for i = statId + 1; i <= Board.StatCount; i++ {
		Board.Stats[i-1] = Board.Stats[i]
	}
	Board.StatCount--
}

func GetStatIdAt(x, y int16) (GetStatIdAt int16) {
	var i int16
	i = -1
	for {
		i++
		if int16(Board.Stats[i].X) == x && int16(Board.Stats[i].Y) == y || i > Board.StatCount {
			break
		}
	}
	if i > Board.StatCount {
		GetStatIdAt = -1
	} else {
		GetStatIdAt = i
	}
	return
}

func BoardPrepareTileForPlacement(x, y int16) (BoardPrepareTileForPlacement bool) {
	var (
		statId int16
		result bool
	)
	statId = GetStatIdAt(x, y)
	if statId > 0 {
		RemoveStat(statId)
		result = true
	} else if statId < 0 {
		if !ElementDefs[Board.Tiles[x][y].Element].PlaceableOnTop {
			Board.Tiles[x][y].Element = E_EMPTY
		}
		result = true
	} else {
		result = false
	}

	BoardDrawTile(x, y)
	BoardPrepareTileForPlacement = result
	return
}

func MoveStat(statId int16, newX, newY int16) {
	var (
		iUnder     TTile
		ix, iy     int16
		oldX, oldY int16
	)
	stat := &Board.Stats[statId]
	iUnder = Board.Stats[statId].Under
	Board.Stats[statId].Under = Board.Tiles[newX][newY]
	if Board.Tiles[stat.X][stat.Y].Element == E_PLAYER {
		Board.Tiles[newX][newY].Color = Board.Tiles[stat.X][stat.Y].Color
	} else if Board.Tiles[newX][newY].Element == E_EMPTY {
		Board.Tiles[newX][newY].Color = byte(int16(Board.Tiles[stat.X][stat.Y].Color) & 0x0F)
	} else {
		Board.Tiles[newX][newY].Color = byte(int16(Board.Tiles[stat.X][stat.Y].Color)&0x0F + int16(Board.Tiles[newX][newY].Color)&0x70)
	}

	Board.Tiles[newX][newY].Element = Board.Tiles[stat.X][stat.Y].Element
	Board.Tiles[stat.X][stat.Y] = iUnder
	oldX = int16(stat.X)
	oldY = int16(stat.Y)
	stat.X = byte(newX)
	stat.Y = byte(newY)
	BoardDrawTile(int16(stat.X), int16(stat.Y))
	BoardDrawTile(oldX, oldY)
	if statId == 0 && Board.Info.IsDark && World.Info.TorchTicks > 0 {
		if Sqr(oldX-int16(stat.X))+Sqr(oldY-int16(stat.Y)) == 1 {
			for ix = int16(stat.X) - TORCH_DX - 3; ix <= int16(stat.X)+TORCH_DX+3; ix++ {
				if ix >= 1 && ix <= BOARD_WIDTH {
					for iy = int16(stat.Y) - TORCH_DY - 3; iy <= int16(stat.Y)+TORCH_DY+3; iy++ {
						if iy >= 1 && iy <= BOARD_HEIGHT {
							if Sqr(ix-oldX)+Sqr(iy-oldY)*2 < TORCH_DIST_SQR != (Sqr(ix-newX)+Sqr(iy-newY)*2 < TORCH_DIST_SQR) {
								BoardDrawTile(ix, iy)
							}
						}
					}
				}
			}
		} else {
			DrawPlayerSurroundings(oldX, oldY, 0)
			DrawPlayerSurroundings(int16(stat.X), int16(stat.Y), 0)
		}
	}
}

func PopupPromptString(question string, buffer *string) {
	var x, y int16
	VideoWriteText(3, 18, 0x4F, TextWindowStrTop)
	VideoWriteText(3, 19, 0x4F, TextWindowStrText)
	VideoWriteText(3, 20, 0x4F, TextWindowStrSep)
	VideoWriteText(3, 21, 0x4F, TextWindowStrText)
	VideoWriteText(3, 22, 0x4F, TextWindowStrText)
	VideoWriteText(3, 23, 0x4F, TextWindowStrBottom)
	VideoWriteText(4+(TextWindowWidth-Length(question))/2, 19, 0x4F, question)
	*buffer = ""
	PromptString(10, 22, 0x4F, 0x4E, TextWindowWidth-16, PROMPT_ANY, buffer)
	for y = 18; y <= 23; y++ {
		for x = 3; x <= TextWindowWidth+3; x++ {
			BoardDrawTile(x+1, y+1)
		}
	}
}

func Signum(val int16) (Signum int16) {
	if val > 0 {
		Signum = 1
	} else if val < 0 {
		Signum = -1
	} else {
		Signum = 0
	}
	return
}

func Difference(a, b int16) (Difference int16) {
	if a-b >= 0 {
		Difference = a - b
	} else {
		Difference = b - a
	}
	return
}

func GameUpdateSidebar() {
	var (
		numStr string
		i      int16
	)
	if GameStateElement == E_PLAYER {
		if Board.Info.TimeLimitSec > 0 {
			VideoWriteText(64, 6, 0x1E, "   Time:")
			numStr = Str(Board.Info.TimeLimitSec - World.Info.BoardTimeSec)
			VideoWriteText(72, 6, 0x1E, numStr+" ")
		} else {
			SidebarClearLine(6)
		}
		if World.Info.Health < 0 {
			World.Info.Health = 0
		}
		numStr = Str(World.Info.Health)
		VideoWriteText(72, 7, 0x1E, numStr+" ")
		numStr = Str(World.Info.Ammo)
		VideoWriteText(72, 8, 0x1E, numStr+"  ")
		numStr = Str(World.Info.Torches)
		VideoWriteText(72, 9, 0x1E, numStr+" ")
		numStr = Str(World.Info.Gems)
		VideoWriteText(72, 10, 0x1E, numStr+" ")
		numStr = Str(World.Info.Score)
		VideoWriteText(72, 11, 0x1E, numStr+" ")
		if World.Info.TorchTicks == 0 {
			VideoWriteText(75, 9, 0x16, "    ")
		} else {
			for i = 2; i <= 5; i++ {
				if i <= World.Info.TorchTicks*5/TORCH_DURATION {
					VideoWriteText(73+i, 9, 0x16, "\xb1")
				} else {
					VideoWriteText(73+i, 9, 0x16, "\xb0")
				}
			}
		}
		for i = 1; i <= 7; i++ {
			if World.Info.Keys[i-1] {
				VideoWriteText(71+i, 12, byte(0x18+i), string([]byte{ElementDefs[E_KEY].Character}))
			} else {
				VideoWriteText(71+i, 12, 0x1F, " ")
			}
		}
		if SoundEnabled {
			VideoWriteText(65, 15, 0x1F, " Be quiet")
		} else {
			VideoWriteText(65, 15, 0x1F, " Be noisy")
		}
	}
}

func DisplayMessage(time int16, message string) {
	if GetStatIdAt(0, 0) != -1 {
		RemoveStat(GetStatIdAt(0, 0))
		BoardDrawBorder()
	}
	if Length(message) != 0 {
		AddStat(0, 0, E_MESSAGE_TIMER, 0, 1, StatTemplateDefault)
		Board.Stats[Board.StatCount].P2 = byte(time / (TickTimeDuration + 1))
		Board.Info.Message = message
	}
}

func DamageStat(attackerStatId int16) {
	var oldX, oldY int16
	stat := &Board.Stats[attackerStatId]
	if attackerStatId == 0 {
		if World.Info.Health > 0 {
			World.Info.Health -= 10
			GameUpdateSidebar()
			DisplayMessage(100, "Ouch!")
			Board.Tiles[stat.X][stat.Y].Color = byte(0x70 + int16(ElementDefs[E_PLAYER].Color)%0x10)
			if World.Info.Health > 0 {
				World.Info.BoardTimeSec = 0
				if Board.Info.ReenterWhenZapped {
					SoundQueue(4, " \x01#\x01'\x010\x01\x10\x01")
					Board.Tiles[stat.X][stat.Y].Element = E_EMPTY
					BoardDrawTile(int16(stat.X), int16(stat.Y))
					oldX = int16(stat.X)
					oldY = int16(stat.Y)
					stat.X = Board.Info.StartPlayerX
					stat.Y = Board.Info.StartPlayerY
					DrawPlayerSurroundings(oldX, oldY, 0)
					DrawPlayerSurroundings(int16(stat.X), int16(stat.Y), 0)
					GamePaused = true
				}
				SoundQueue(4, "\x10\x01 \x01\x13\x01#\x01")
			} else {
				SoundQueue(5, " \x03#\x03'\x030\x03'\x03*\x032\x037\x035\x038\x03@\x03E\x03\x10\n")
			}
		}
	} else {
		switch Board.Tiles[stat.X][stat.Y].Element {
		case E_BULLET:
			SoundQueue(3, " \x01")
		case E_OBJECT:
		default:
			SoundQueue(3, "@\x01\x10\x01P\x010\x01")
		}
		RemoveStat(attackerStatId)
	}
}

func BoardDamageTile(x, y int16) {
	var statId int16
	statId = GetStatIdAt(x, y)
	if statId != -1 {
		DamageStat(statId)
	} else {
		Board.Tiles[x][y].Element = E_EMPTY
		BoardDrawTile(x, y)
	}
}

func BoardAttack(attackerStatId int16, x, y int16) {
	if attackerStatId == 0 && World.Info.EnergizerTicks > 0 {
		World.Info.Score = ElementDefs[Board.Tiles[x][y].Element].ScoreValue + World.Info.Score
		GameUpdateSidebar()
	} else {
		DamageStat(attackerStatId)
	}
	if attackerStatId > 0 && attackerStatId <= CurrentStatTicked {
		CurrentStatTicked--
	}
	if Board.Tiles[x][y].Element == E_PLAYER && World.Info.EnergizerTicks > 0 {
		World.Info.Score = ElementDefs[Board.Tiles[Board.Stats[attackerStatId].X][Board.Stats[attackerStatId].Y].Element].ScoreValue + World.Info.Score
		GameUpdateSidebar()
	} else {
		BoardDamageTile(x, y)
		SoundQueue(2, "\x10\x01")
	}
}

func BoardShoot(element byte, tx, ty, deltaX, deltaY int16, source int16) (BoardShoot bool) {
	if ElementDefs[Board.Tiles[tx+deltaX][ty+deltaY].Element].Walkable || Board.Tiles[tx+deltaX][ty+deltaY].Element == E_WATER {
		AddStat(tx+deltaX, ty+deltaY, element, int16(ElementDefs[element].Color), 1, StatTemplateDefault)
		stat := &Board.Stats[Board.StatCount]
		stat.P1 = byte(source)
		stat.StepX = deltaX
		stat.StepY = deltaY
		stat.P2 = 100
		BoardShoot = true
	} else if Board.Tiles[tx+deltaX][ty+deltaY].Element == E_BREAKABLE || ElementDefs[Board.Tiles[tx+deltaX][ty+deltaY].Element].Destructible && Board.Tiles[tx+deltaX][ty+deltaY].Element == E_PLAYER == (source != 0) && World.Info.EnergizerTicks <= 0 {
		BoardDamageTile(tx+deltaX, ty+deltaY)
		SoundQueue(2, "\x10\x01")
		BoardShoot = true
	} else {
		BoardShoot = false
	}

	return
}

func CalcDirectionRnd(deltaX, deltaY *int16) {
	*deltaX = Random(3) - 1
	if *deltaX == 0 {
		*deltaY = Random(2)*2 - 1
	} else {
		*deltaY = 0
	}
}

func CalcDirectionSeek(x, y int16, deltaX, deltaY *int16) {
	*deltaX = 0
	*deltaY = 0
	if Random(2) < 1 || int16(Board.Stats[0].Y) == y {
		*deltaX = Signum(int16(Board.Stats[0].X) - x)
	}
	if *deltaX == 0 {
		*deltaY = Signum(int16(Board.Stats[0].Y) - y)
	}
	if World.Info.EnergizerTicks > 0 {
		*deltaX = -*deltaX
		*deltaY = -*deltaY
	}
}

func TransitionDrawBoardChange() {
	TransitionDrawToFill('\xdb', 0x05)
	TransitionDrawToBoard()
}

func BoardEnter() {
	Board.Info.StartPlayerX = Board.Stats[0].X
	Board.Info.StartPlayerY = Board.Stats[0].Y
	if Board.Info.IsDark && MessageHintTorchNotShown {
		DisplayMessage(200, "Room is dark - you need to light a torch!")
		MessageHintTorchNotShown = false
	}
	World.Info.BoardTimeSec = 0
	GameUpdateSidebar()
}

func BoardPassageTeleport(x, y int16) {
	var (
		col        byte
		ix, iy     int16
		newX, newY int16
	)
	col = Board.Tiles[x][y].Color
	BoardChange(int16(Board.Stats[GetStatIdAt(x, y)].P3))
	newX = 0
	for ix = 1; ix <= BOARD_WIDTH; ix++ {
		for iy = 1; iy <= BOARD_HEIGHT; iy++ {
			if Board.Tiles[ix][iy].Element == E_PASSAGE && Board.Tiles[ix][iy].Color == col {
				newX = ix
				newY = iy
			}
		}
	}
	Board.Tiles[Board.Stats[0].X][Board.Stats[0].Y].Element = E_EMPTY
	Board.Tiles[Board.Stats[0].X][Board.Stats[0].Y].Color = 0
	if newX != 0 {
		Board.Stats[0].X = byte(newX)
		Board.Stats[0].Y = byte(newY)
	}
	GamePaused = true
	SoundQueue(4, "0\x014\x017\x011\x015\x018\x012\x016\x019\x013\x017\x01:\x014\x018\x01@\x01")
	TransitionDrawBoardChange()
	BoardEnter()
}

func GameDebugPrompt() {
	var (
		input  string
		i      int16
		toggle bool
	)
	input = ""
	SidebarClearLine(4)
	SidebarClearLine(5)
	PromptString(63, 5, 0x1E, 0x0F, 11, PROMPT_ANY, &input)
	input = UpCaseString(input)
	toggle = true
	if input[0] == '+' || input[0] == '-' {
		if input[0] == '-' {
			toggle = false
		}
		input = Copy(input, 2, Length(input)-1)
		if toggle == true {
			WorldSetFlag(input)
		} else {
			WorldClearFlag(input)
		}
	}
	DebugEnabled = WorldGetFlagPosition("DEBUG") >= 0
	if input == "HEALTH" {
		World.Info.Health += 50
	} else if input == "AMMO" {
		World.Info.Ammo += 5
	} else if input == "KEYS" {
		for i = 1; i <= 7; i++ {
			World.Info.Keys[i-1] = true
		}
	} else if input == "TORCHES" {
		World.Info.Torches += 3
	} else if input == "TIME" {
		World.Info.BoardTimeSec -= 30
	} else if input == "GEMS" {
		World.Info.Gems += 5
	} else if input == "DARK" {
		Board.Info.IsDark = toggle
		TransitionDrawToBoard()
	} else if input == "ZAP" {
		for i = 0; i <= 3; i++ {
			BoardDamageTile(int16(Board.Stats[0].X)+NeighborDeltaX[i], int16(Board.Stats[0].Y)+NeighborDeltaY[i])
			Board.Tiles[int16(Board.Stats[0].X)+NeighborDeltaX[i]][int16(Board.Stats[0].Y)+NeighborDeltaY[i]].Element = E_EMPTY
			BoardDrawTile(int16(Board.Stats[0].X)+NeighborDeltaX[i], int16(Board.Stats[0].Y)+NeighborDeltaY[i])
		}
	}

	SoundQueue(10, "'\x04")
	SidebarClearLine(4)
	SidebarClearLine(5)
	GameUpdateSidebar()
}

func GameAboutScreen() {
	TextWindowDisplayFile("ABOUT.HLP", "About ZZT...")
}

func GamePlayLoop(boardChanged bool) {
	var pauseBlink bool

	GameDrawSidebar := func() {
		SidebarClear()
		SidebarClearLine(0)
		SidebarClearLine(1)
		SidebarClearLine(2)
		VideoWriteText(61, 0, 0x1F, "    - - - - -      ")
		VideoWriteText(62, 1, 0x70, "      ZZT      ")
		VideoWriteText(61, 2, 0x1F, "    - - - - -      ")
		if GameStateElement == E_PLAYER {
			VideoWriteText(64, 7, 0x1E, " Health:")
			VideoWriteText(64, 8, 0x1E, "   Ammo:")
			VideoWriteText(64, 9, 0x1E, "Torches:")
			VideoWriteText(64, 10, 0x1E, "   Gems:")
			VideoWriteText(64, 11, 0x1E, "  Score:")
			VideoWriteText(64, 12, 0x1E, "   Keys:")
			VideoWriteText(62, 7, 0x1F, string([]byte{ElementDefs[E_PLAYER].Character}))
			VideoWriteText(62, 8, 0x1B, string([]byte{ElementDefs[E_AMMO].Character}))
			VideoWriteText(62, 9, 0x16, string([]byte{ElementDefs[E_TORCH].Character}))
			VideoWriteText(62, 10, 0x1B, string([]byte{ElementDefs[E_GEM].Character}))
			VideoWriteText(62, 12, 0x1F, string([]byte{ElementDefs[E_KEY].Character}))
			VideoWriteText(62, 14, 0x70, " T ")
			VideoWriteText(65, 14, 0x1F, " Torch")
			VideoWriteText(62, 15, 0x30, " B ")
			VideoWriteText(62, 16, 0x70, " H ")
			VideoWriteText(65, 16, 0x1F, " Help")
			VideoWriteText(67, 18, 0x30, " \x18\x19\x1a\x1b ")
			VideoWriteText(72, 18, 0x1F, " Move")
			VideoWriteText(61, 19, 0x70, " Shift \x18\x19\x1a\x1b ")
			VideoWriteText(72, 19, 0x1F, " Shoot")
			VideoWriteText(62, 21, 0x70, " S ")
			VideoWriteText(65, 21, 0x1F, " Save game")
			VideoWriteText(62, 22, 0x30, " P ")
			VideoWriteText(65, 22, 0x1F, " Pause")
			VideoWriteText(62, 23, 0x70, " Q ")
			VideoWriteText(65, 23, 0x1F, " Quit")
		} else if GameStateElement == E_MONITOR {
			SidebarPromptSlider(false, 66, 21, "Game speed:;FS", &TickSpeed)
			VideoWriteText(62, 21, 0x70, " S ")
			VideoWriteText(62, 7, 0x30, " W ")
			VideoWriteText(65, 7, 0x1E, " World:")
			if Length(World.Info.Name) != 0 {
				VideoWriteText(69, 8, 0x1F, World.Info.Name)
			} else {
				VideoWriteText(69, 8, 0x1F, "Untitled")
			}
			VideoWriteText(62, 11, 0x70, " P ")
			VideoWriteText(65, 11, 0x1F, " Play")
			VideoWriteText(62, 12, 0x30, " R ")
			VideoWriteText(65, 12, 0x1E, " Restore game")
			VideoWriteText(62, 13, 0x70, " Q ")
			VideoWriteText(65, 13, 0x1E, " Quit")
			VideoWriteText(62, 16, 0x30, " A ")
			VideoWriteText(65, 16, 0x1F, " About ZZT!")
			VideoWriteText(62, 17, 0x70, " H ")
			VideoWriteText(65, 17, 0x1E, " High Scores")
			if EditorEnabled {
				VideoWriteText(62, 18, 0x30, " E ")
				VideoWriteText(65, 18, 0x1E, " Board Editor")
			}
		}
	}

	GameDrawSidebar()
	GameUpdateSidebar()

	if JustStarted {
		// TODO: GameAboutScreen()
		if Length(StartupWorldFileName) != 0 {
			SidebarClearLine(8)
			VideoWriteText(69, 8, 0x1F, StartupWorldFileName)
			if !WorldLoad(StartupWorldFileName, ".ZZT", true) {
				WorldCreate()
			}
		}
		ReturnBoardId = World.Info.CurrentBoard
		BoardChange(0)
		JustStarted = false
	}

	Board.Tiles[Board.Stats[0].X][Board.Stats[0].Y].Element = byte(GameStateElement)
	Board.Tiles[Board.Stats[0].X][Board.Stats[0].Y].Color = ElementDefs[GameStateElement].Color
	if GameStateElement == E_MONITOR {
		DisplayMessage(0, "")
		VideoWriteText(62, 5, 0x1B, "Pick a command:")
	}
	if boardChanged {
		TransitionDrawBoardChange()
	}
	TickTimeDuration = int16(TickSpeed) * 2
	GamePlayExitRequested = false
	CurrentTick = Random(100)
	CurrentStatTicked = Board.StatCount + 1

	for !GamePlayExitRequested {
		if GamePaused {
			if SoundHasTimeElapsed(&TickTimeCounter, 25) {
				pauseBlink = !pauseBlink
			}
			if pauseBlink {
				VideoWriteText(int16(Board.Stats[0].X)-1, int16(Board.Stats[0].Y)-1, ElementDefs[E_PLAYER].Color, string([]byte{ElementDefs[E_PLAYER].Character}))
			} else {
				if Board.Tiles[Board.Stats[0].X][Board.Stats[0].Y].Element == E_PLAYER {
					VideoWriteText(int16(Board.Stats[0].X)-1, int16(Board.Stats[0].Y)-1, 0x0F, " ")
				} else {
					BoardDrawTile(int16(Board.Stats[0].X), int16(Board.Stats[0].Y))
				}
			}
			VideoWriteText(64, 5, 0x1F, "Pausing...")

			InputUpdate()
			if InputKeyPressed == KEY_ESCAPE {
				GamePromptEndPlay()
			}
			if InputDeltaX != 0 || InputDeltaY != 0 {
				ElementDefs[Board.Tiles[int16(Board.Stats[0].X)+InputDeltaX][int16(Board.Stats[0].Y)+InputDeltaY].Element].TouchProc(int16(Board.Stats[0].X)+InputDeltaX, int16(Board.Stats[0].Y)+InputDeltaY, 0, &InputDeltaX, &InputDeltaY)
			}
			if (InputDeltaX != 0 || InputDeltaY != 0) && ElementDefs[Board.Tiles[int16(Board.Stats[0].X)+InputDeltaX][int16(Board.Stats[0].Y)+InputDeltaY].Element].Walkable {
				// Move player
				if Board.Tiles[Board.Stats[0].X][Board.Stats[0].Y].Element == E_PLAYER {
					MoveStat(0, int16(Board.Stats[0].X)+InputDeltaX, int16(Board.Stats[0].Y)+InputDeltaY)
				} else {
					BoardDrawTile(int16(Board.Stats[0].X), int16(Board.Stats[0].Y))
					Board.Stats[0].X += byte(InputDeltaX)
					Board.Stats[0].Y += byte(InputDeltaY)
					Board.Tiles[Board.Stats[0].X][Board.Stats[0].Y].Element = E_PLAYER
					Board.Tiles[Board.Stats[0].X][Board.Stats[0].Y].Color = ElementDefs[E_PLAYER].Color
					BoardDrawTile(int16(Board.Stats[0].X), int16(Board.Stats[0].Y))
					DrawPlayerSurroundings(int16(Board.Stats[0].X), int16(Board.Stats[0].Y), 0)
					DrawPlayerSurroundings(int16(Board.Stats[0].X)-InputDeltaX, int16(Board.Stats[0].Y)-InputDeltaY, 0)
				}

				// Unpause
				GamePaused = false
				SidebarClearLine(5)
				CurrentTick = Random(100)
				CurrentStatTicked = Board.StatCount + 1
				World.Info.IsSave = true
			}
		} else { // not GamePaused
			if CurrentStatTicked <= Board.StatCount {
				stat := &Board.Stats[CurrentStatTicked]
				if stat.Cycle != 0 && CurrentTick%stat.Cycle == CurrentStatTicked%stat.Cycle {
					ElementDefs[Board.Tiles[stat.X][stat.Y].Element].TickProc(CurrentStatTicked)
				}
				CurrentStatTicked++
			}
		}

		if CurrentStatTicked > Board.StatCount && !GamePlayExitRequested {
			// all stats ticked

			// TODO: should wait till next TickTimeCounter/TickTimeDuration up
			time.Sleep(time.Duration(TickTimeDuration) * 10 * time.Millisecond)

			if SoundHasTimeElapsed(&TickTimeCounter, TickTimeDuration) {
				// next cycle
				CurrentTick++
				if CurrentTick > 420 {
					CurrentTick = 1
				}
				CurrentStatTicked = 0
				InputUpdate()
			}
		}
	}

	SoundClearQueue()
	if GameStateElement == E_PLAYER {
		if World.Info.Health <= 0 {
			HighScoresAdd(World.Info.Score)
		}
	} else if GameStateElement == E_MONITOR {
		SidebarClearLine(5)
	}

	Board.Tiles[Board.Stats[0].X][Board.Stats[0].Y].Element = E_PLAYER
	Board.Tiles[Board.Stats[0].X][Board.Stats[0].Y].Color = ElementDefs[E_PLAYER].Color
	SoundBlockQueueing = false
}

func GameTitleLoop() {
	var (
		boardChanged bool
		startPlay    bool
	)
	GameTitleExitRequested = false
	JustStarted = true
	ReturnBoardId = 0
	boardChanged = true
	for {
		BoardChange(0)
		for {
			GameStateElement = E_MONITOR
			startPlay = false
			GamePaused = false
			GamePlayLoop(boardChanged)
			boardChanged = false
			switch UpCase(InputKeyPressed) {
			case 'W':
				if GameWorldLoad(".ZZT") {
					ReturnBoardId = World.Info.CurrentBoard
					boardChanged = true
				}
			case 'P':
				if World.Info.IsSave && !DebugEnabled {
					startPlay = WorldLoad(World.Info.Name, ".ZZT", false)
					ReturnBoardId = World.Info.CurrentBoard
				} else {
					startPlay = true
				}
				if startPlay {
					BoardChange(ReturnBoardId)
					BoardEnter()
				}
			case 'A':
				GameAboutScreen()
			case 'E':
				if EditorEnabled {
					EditorLoop()
					ReturnBoardId = World.Info.CurrentBoard
					boardChanged = true
				}
			case 'S':
				SidebarPromptSlider(true, 66, 21, "Game speed:;FS", &TickSpeed)
				InputKeyPressed = '\x00'
			case 'R':
				if GameWorldLoad(".SAV") {
					ReturnBoardId = World.Info.CurrentBoard
					BoardChange(ReturnBoardId)
					startPlay = true
				}
			case 'H':
				HighScoresLoad()
				HighScoresDisplay(1)
			case '|':
				GameDebugPrompt()
			case KEY_ESCAPE, 'Q':
				GameTitleExitRequested = SidebarPromptYesNo("Quit ZZT? ", true)
			}
			if startPlay {
				GameStateElement = E_PLAYER
				GamePaused = true
				GamePlayLoop(true)
				boardChanged = true
			}
			if boardChanged || GameTitleExitRequested {
				break
			}
		}
		if GameTitleExitRequested {
			break
		}
	}
}
