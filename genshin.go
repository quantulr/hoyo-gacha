package main

// loadGenshinData 从应用数据目录读取原神的抽卡数据。
func loadGenshinData() (*gachaData, error) {
	return loadGameData("genshin")
}
