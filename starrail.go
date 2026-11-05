package main

// loadStarRailData 从应用数据目录读取崩坏：星穹铁道的抽卡数据。
func loadStarRailData() (*gachaData, error) {
	return loadGameData("starrail")
}
