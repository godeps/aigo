// Package aliyun 对接阿里云百炼（DashScope）多模态 API。
//
// 对外入口为本目录下的 [Config]、[Engine]、[New] 与模型常量；实现按能力域拆在 internal 子包中（仅 aliyun 可导入）：
//   - internal/imggen     图片生成（qwen-image 文生图、Wan/z-image 多模态图）
//   - internal/vidgen     视频生成与编辑（Wan t2v / r2v / videoedit）
//   - internal/audiogen   语音合成与声音设计（Qwen TTS、qwen-voice-design）
//   - internal/async      异步任务创建与轮询
//   - internal/graphx     从 workflow.Graph 抽取各域共用字段
//   - internal/runtime    HTTP 与轮询等运行时参数
//   - internal/ierr       错误哨兵（根包重新导出以保持 API 稳定）
package aliyun
