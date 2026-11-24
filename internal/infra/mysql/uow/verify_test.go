package uow

import appuow "github.com/guliangiam/guliangiam/internal/app/uow"

// 编译期接口断言：UnitOfWork 必须完整实现 appuow.UnitOfWork。
// 如果 mysqluow 漏实现了任何方法，这里会在 go build 阶段报错。
var _ appuow.UnitOfWork = UnitOfWork{}
