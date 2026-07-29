# 音频分析候选与基准协议

最后复核：2026-07-29

本文件记录 M5.3 的候选筛选、许可证边界和本地可复现评估协议。它不是法律意见；正式分发前仍需对**实际镜像、代码版本和权重文件**逐项复核。

## 结论

Music Online 目前不内置、不自动下载、也不默认启动任何音频模型。默认能力继续是离线元数据规则；HTTP analyzer 只是可选适配器。

首轮真实曲库实验建议保留两条路线：

1. 低成本路线：轻量 AudioSet tagger（优先考察 EfficientAT，其次 PANNs/YAMNet）提供“人声、电子音乐、Dubstep、节拍/冲击”等弱证据，再结合本地 DSP 特征。
2. 开放词表路线：固定版本的 LAION CLAP 用预设描述词做零样本打分，用来验证细粒度风格是否值得其明显更高的镜像、内存和 CPU 成本。

在私有 gold set 的留出集达到约定指标、资源预算和许可证门槛前，不选默认模型，不声称准确率，也不把候选镜像纳入正式 Compose 默认路径。

## 候选面

| 候选 | 优点 | 与四类预设的缺口 | 权利与运维结论 |
|---|---|---|---|
| 元数据规则 + DSP | 无权重、CPU 可控、证据最容易解释；断网和 analyzer 关闭时仍可工作 | DSP 不能单独可靠识别 Chillstep、Complextro 等语义风格 | 继续作为默认基线；DSP 可用固定版本的 [librosa](https://librosa.org/doc/latest/feature.html) 或等价实现放在可选 sidecar 中，库本身为 [ISC](https://github.com/librosa/librosa/blob/main/LICENSE.md) |
| [EfficientAT](https://github.com/fschmid56/EfficientAT) | 面向资源受限设备的轻量 CNN；提供不同复杂度的 AudioSet 模型 | AudioSet 标签以声音事件和大类为主，无法直接覆盖多数细分电子风格 | 仓库为 MIT；采用前仍需固定具体 checkpoint、校验权重来源与摘要。作为首选轻量实验候选，不直接作为真值 |
| [PANNs](https://github.com/qiuqiangkong/audioset_tagging_cnn) / [YAMNet](https://www.tensorflow.org/hub/tutorials/yamnet) | 成熟的 521/527 类 AudioSet 基线，能提供人声、音乐和部分电子音乐证据 | 粒度仍不足，PANNs 运行栈较旧且大模型 CPU 成本更高 | PANNs 代码为 MIT；具体 Zenodo 权重仍单独记录许可证。YAMNet 同样只作弱证据 |
| [musicnn](https://github.com/jordipons/musicnn) | 面向音乐标签，代码为 ISC，包含 MTT/MSD 预训练模型 | 50 标签词表太粗，且 PyPI 版本仍依赖 TensorFlow 1.x 时代栈 | 只作遗留基线，不进入默认镜像 |
| [LAION CLAP](https://github.com/LAION-AI/CLAP) | 音频—文本相似度支持开放描述词，不必先训练固定四类 head | 模型大；官方预处理通常只看有限片段，必须做多片段聚合；零样本分数需校准 | 仅实验性可选候选。必须固定具体模型卡、权重 SHA-256 和镜像 digest；训练数据与权重权利不能只从代码许可证推断 |
| [OpenL3](https://github.com/marl/openl3) | MIT 代码、CC BY 4.0 权重；有 music embedding | 只输出 embedding，仍需本地标注集训练和版本化分类头 | 可作为训练轻量 head 的后备方案；TensorFlow/libsndfile 增加镜像成本 |
| [MERT](https://github.com/yizhilll/MERT)、[MusicFM](https://github.com/minzwon/musicfm)、[MuQ](https://github.com/tencent-ailab/MuQ) | 音乐专用表示，适合 beat、结构、标签等下游任务 | 都不是可直接消费的四类分类器；模型大，通常需要 GPU 或训练 probe/head | 研究候选。MERT 代码 Apache-2.0，但部分模型卡为非商业许可；MuQ 权重明确为 CC BY-NC 4.0；MusicFM 的 FMA/MSD checkpoint 必须分别审查，不随代码许可证推断 |
| [Essentia](https://essentia.upf.edu/) 与 MTG 模型 | DSP/MIR 特征丰富，已有大量音乐模型 | 运行时和模型矩阵较复杂，模型标签仍未必覆盖细分边界 | [核心许可](https://essentia.upf.edu/licensing_information.html)和[模型许可](https://essentia.upf.edu/models.html)存在 AGPL、非商业或需商业授权的组合；正式镜像不打包，只有管理员自备并完成逐文件审查后才能接入 |

候选表只说明技术适配性，不替代权利审查。仓库中的 `code_license` 和 `model_license` 字段是审计记录，不会自动证明许可证兼容。

## Gold set 设计

- 每个预设至少 30–50 首人工确认曲目，并加入数量足够的 `none` 负样本。
- 包含融合风格、长前奏、half-time/double-time、纯音乐但高能量、低能量但有人声等困难样本。
- 同一艺术家、专辑、版本或近重复音源只能位于一个 split；`groups` 是防泄漏分组标识而不是普通描述标签，工具会拒绝任何跨越 `calibration`/`evaluation` 的同名组。
- `calibration` 只用于阈值、权重、描述词和片段策略选择；所有对外报告只读取未参与调参的 `evaluation`。
- 清单中的 `audio_ref` 是部署者本机的相对引用。版权音频、私有绝对路径和用户信息不得提交到仓库或公开 CI。
- 清单 revision 固化后不就地改标签；修订时创建新 revision。结果文件绑定清单原始字节的 SHA-256，避免误用另一版标签集。

示例清单只是 JSON 契约 smoke test。真实清单的两个 split 均必须至少包含四类和 `none`；工具会拒绝缺失类别、重复 ID、未知字段、非有限分数和候选漏项。

## 候选运行结果

每个候选结果必须记录：

- 实现、模型和规则包版本；模型文件 SHA-256；容器必须使用 `image@sha256:...`。
- 代码与权重许可证的原文标识；无法确认时写明 `UNKNOWN` 并判定不可发布。
- 每首曲目的四类 `0..1` 分数，或稳定、无敏感信息的 `error_code`。
- 定义一致的进程 CPU 时间和峰值内存；容器镜像大小与同一引擎、同一基础镜像的增量。

建议每个候选先 warm-up，再对每首曲目重复运行三次：CPU 时间取中位数，内存取最大值。片段策略也属于候选版本的一部分；“全曲低成本统计 + 开头/中段/高能量段的固定窗口”比只取开头更符合本项目目标。

基准工具报告每类 precision/recall/F1、macro-F1、高置信度 precision 与样本数、覆盖率/弃权率、失败率、混淆矩阵、平均/p95 CPU、峰值内存和镜像增量。资源数值由候选 runner 测量，聚合器不会伪造或推断缺失数据。

## 使用方式

复制示例，换成本机清单和候选 runner 的输出：

```bash
make benchmark-analysis ARGS="-manifest private/gold-set-v1.json -result private/rules-v1.json -result private/efficientat-v1.json -format markdown"
```

仓库内示例可用于验证 schema 和报告格式：

```bash
make benchmark-analysis ARGS="-manifest docs/analysis-benchmark/gold-set.example.json -result docs/analysis-benchmark/candidate-result.example.json -format markdown"
```

示例分数由人工构造，不能作为任何模型或规则的准确率证据。

## 进入正式可选实现的门槛

1. 具体代码、模型、数据衍生限制和再分发权利已记录并可接受。
2. 镜像、权重和依赖均固定 digest，生成 SBOM 并通过漏洞扫描。
3. 私有 evaluation 留出集报告可复现，且没有用它调阈值、描述词或 head。
4. 高置信度 precision、自动覆盖率和每类召回达到项目实际目标；不能用单一总准确率掩盖小类失败。
5. CPU、内存、单曲耗时和镜像增量适合个人/家庭服务器；无 GPU 部署仍有清晰退化路径。
6. analyzer 离线、超时或失败时，上传、目录导入和播放保持正常。
