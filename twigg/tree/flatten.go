package tree

import "io"

type flatten struct {
	tr Tree
}

func (ft flatten) IsRemovedChild() bool {
	return ft.tr.IsRemovedChild()
}
func (ft flatten) DataIsComplete() bool {
	return ft.tr.DataIsComplete()
}
func (ft flatten) Data() Data {
	d := ft.tr.Data()
	d.HasChildrenData = false
	d.ChildrenData = nil
	d.ChildrenDataIsComplete = nil
	return d
}
func (ft flatten) GetFile() (wt io.WriterTo, err error) {
	return ft.tr.GetFile()
}
