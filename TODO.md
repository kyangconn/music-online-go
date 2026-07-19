# Music Online Roadmap

## 产品主旨

Music Online 是一个面向个人、家庭或小团队的 **self-hosted 小型音乐平台**：用一个实例管理、检索和播放自己的音乐库，并兼容常见的 MusicBrainz Picard 标签。

- **Self-hosted**：单机、单容器或 Compose 即可部署；SQLite 是默认路径，PostgreSQL 是可选路径；备份、迁移和升级必须可控。
- **MusicBrainz 标签兼容**：Picard 标记过的文件在“浏览器解析 → API → 数据库 → 查看/编辑”链路中不应丢失常用标签和稳定 ID。
- **小型平台**：优先做好音乐库、播放、导入和少量用户协作，不向商业流媒体、社交网络或多节点媒体集群扩张。
- **离线优先**：本地音乐的导入和播放不能依赖 MusicBrainz 在线服务；在线查询只负责可选的补全和校对。

第一阶段的“标签兼容”不包含回写或修改源音频文件，也不代表完整镜像 MusicBrainz 数据库。标签基线以 [Picard Tag Mapping](https://picard-docs.musicbrainz.org/en/latest/appendices/tag_mapping.html) 为准。

现有的上传、播放、批量导入、重复检测、用户管理、TOTP、Docker/Compose、备份恢复、健康检查和 PWA 等能力已经完成，不再在 TODO 中保留勾选历史；完成记录以 Git 历史和 README 为准。

## M2：可选的 MusicBrainz 在线补全

- [ ] 实现真正的 MusicBrainz Web Service 客户端，通过 recording/release/artist ID 查找，并支持按现有标题、艺术家、专辑搜索候选；当前 `/mbid/lookup` 只查本地表，不能称为在线补全。
- [ ] 所有候选结果先展示差异并由用户确认，默认只补空值，不静默覆盖本地修改；批量操作必须能暂停、取消并逐项报告失败。
- [ ] 遵守 [MusicBrainz API](https://musicbrainz.org/doc/MusicBrainz_API) 约束：有联系方式的 User-Agent、全实例每秒不超过一次请求、超时、退避和缓存；MusicBrainz 不可用时本地功能正常降级。
- [ ] 为服务开关、API 基地址、User-Agent 联系方式、超时和缓存期限提供配置；所有新增配置同时出现在示例 YAML、环境变量文档、Compose 传递说明和配置测试中。
- [ ] 可选接入 Cover Art Archive，候选封面必须由用户确认并复制到受管理的上传目录；远程 URL 不直接成为永久媒体路径。
- [ ] 用固定响应 fixture 测试解析和匹配，不在普通单元测试或 CI 中实时依赖 MusicBrainz。

## M4：让元数据真正服务于浏览和播放

- [ ] 增加艺术家和专辑视图；有 MusicBrainz ID 时用稳定 ID 分组，没有时使用规范化文本回退，并允许播放整张专辑或加入队列。
- [ ] 扩充服务端筛选和 URL 状态，至少覆盖专辑、专辑艺术家、流派和发行年份；分页与访问策略保持一致。
- [ ] 增加用户播放列表：首版只做私有列表、排序、增删曲目和加入播放队列，不引入协作编辑、推荐算法或社交动态。
- [ ] 为无封面、缺标签、同名艺术家/专辑和多碟专辑提供明确的前端状态，并同步中英文 i18n。

## M5：可解释的音乐场景预设分类

该功能是管理员可维护的“听感/使用场景”分类，不是完整流派树，也不是基于用户行为的推荐系统。它必须建立在 M1 的统一元数据主干上：先给每首音乐计算四个独立分数，再产生一个主要预设；证据不足或类别冲突时保留“待确认/未归类”，不能强制四选一。

### 预设定义与首版边界

| 稳定标识 | 首选展示名 | 兼容名称 | 首版锚定风格与听感 |
| --- | --- | --- | --- |
| `calm_flow` | 静谧心流 | 平静纯音乐 | Chillstep、Ambient、Downtempo、Chillout，以及有低能量、低人声概率等证据支持的纯音乐 |
| `kinetic_pulse` | 律动跃迁 | 动感悦动曲 | Complextro、Drum & Bass、Electro House、Melodic Dubstep、Breakbeat、Glitch Hop 等高律动音乐 |
| `cosmic_drift` | 星云漫游 | 星云漫游境 | Trance、Progressive House、Synthwave/Retrowave，以及稳定脉冲、合成器和空间氛围明显的音乐 |
| `bass_impact` | 低频震域 | 震撼我自己 | 大部分普通 Dubstep、Brostep、Riddim、Tearout、Hardstyle/Rawstyle、Gabber 等重低频或高冲击音乐 |

上述稳定标识进入 API 和数据库后不随文案改名；中英文展示名进入 i18n，旧名称可作为中文别名保留。需求中的 `style` 首版暂按 `hardstyle` 理解，正式固化规则前需用真实曲库确认；如果实际指其他 `*style` 子流派，只调整别名/规则，不修改预设标识。

### 可行方案与判定规则

- [ ] 建立可解释的混合分类器，证据优先级为“管理员人工选择 > 规范化流派规则 > 音频模型标签 > DSP 音频特征 > 可选外部大众标签”；人工选择只决定最终展示结果，自动分数仍单独保存，便于以后复核。
- [ ] 为四个预设分别保存 `0..1` 分数、置信度、主要预设、规则版本和证据摘要；达到阈值才自动归类，低置信度、最高两类过于接近或没有有效证据时进入“待确认/未归类”。
- [ ] 在后端实现唯一的流派 tokenizer、规范化器和别名表，兼容大小写、空格、连字符及 `;`、`,`、`/` 等常见分隔方式，同时保留原始标签用于展示；不能让前端拼接格式成为分类规则。
- [ ] 固化并测试交叉风格优先级：`chillstep` 覆盖普通 `dubstep` 进入静谧心流，`melodic dubstep` 进入律动跃迁，未细分的 `dubstep` 默认倾向低频震域，D&B 倾向律动跃迁，Trance 倾向星云漫游。
- [ ] “纯音乐/Instrumental”不能单独推出静谧心流；还需低能量、较平滑动态或低唤醒度等证据，避免把激烈的纯音乐误归为平静。
- [ ] 首批 DSP 特征评估 BPM 及其置信度/候选拍速、danceability、onset rate、pulse clarity/节拍稳定性、响度、动态范围、spectral centroid/flatness/flux、粗糙度代理、低频/次低频能量占比、分段能量与 drop contrast、人声/纯音乐概率，以及可用时的调性/和声特征。
- [ ] BPM 只作弱证据，不能设置单一硬阈值；电子音乐常有 half-time/double-time，Dubstep、D&B、Trance 等也存在跨速和融合风格。
- [ ] 采用“全曲低成本统计 + 代表性/高能量片段模型推理”，避免只分析开头导致前奏误判，也避免默认对整曲运行昂贵模型。
- [ ] MusicBrainz/Last.fm 一类大众标签只能作为可选补充，断网时规则分类和本地播放必须正常；不把已停止持续分析服务的 AcousticBrainz 作为主依赖。

### 分阶段实现

- [ ] **M5.1 元数据基线**：完成预设常量、流派规范化/别名/优先级、四类评分、置信阈值、未归类状态、人工覆盖、证据展示、服务端筛选及单元测试；没有音频分析器时也能依靠标签工作。
- [ ] **M5.2 分析基础设施**：定义与具体模型无关的 HTTP analyzer 契约，增加持久化异步任务、上传后入队、历史曲库回填、单曲重试/重分析、取消、超时、背压和运行指标；音频分析失败不得阻塞上传、导入或播放。
- [ ] **M5.3 模型基准测试**：以 Essentia 及其预训练模型、musicnn、CLAP 等作为候选而非预先钦定依赖，用本地标注集比较标签覆盖、准确率、资源占用、镜像体积、许可证和维护成本，再决定默认/可选实现。
- [ ] **M5.4 校准与产品化**：按基准结果校准各类阈值和权重，增加低置信度审核队列、批量人工修正、预设浏览/播放入口，以及完整的中英文说明。

### 现有代码待改造事项

- [ ] 以 `MediaFile.FileHash` 作为分析缓存键，并结合 `ObservedFileHash`、`ContentRevision` 和元数据修订判断失效：同一内容与分析器/规则版本可复用结果；文件替换后旧结果标记为 `stale` 并重新入队，纯元数据变更只重跑低成本规则层。
- [ ] 通过版本化迁移增加首选数据结构：`music_audio_analyses` 保存文件哈希、状态、分析器/模型版本、特征、模型标签、耗时和错误摘要；`music_preset_classifications` 分开保存自动结果与人工覆盖；`music_preset_scores` 保存四类分数和证据。JSON/Text 字段、索引、唯一约束与删除清理必须同时兼容 SQLite 和 PostgreSQL。
- [ ] 增加持久化任务状态 `pending/running/succeeded/failed/stale`；服务启动时回收孤立的 `running` 任务，默认单并发，并实现有限重试、超时、取消、队列上限和幂等键，不能只用进程内 goroutine 记录状态。
- [ ] 上传文件和数据库事务都成功后才能入队；批量导入复用同一入口。为既有曲库提供管理员显式触发的 backfill，不能在数据库迁移或服务启动时自动扫描全库。
- [ ] 保持现有纯 Go 静态二进制可独立运行；FFmpeg、Python 或模型运行时放入可选的 Compose analyzer profile，通过适配器接入，不能让基础镜像和普通部署被迫携带大型模型。
- [ ] analyzer 只接收经过鉴权和校验的曲目 ID/受控流，或访问只读上传卷；禁止客户端提交任意服务器路径，并限制解码时长、文件大小、CPU、内存和并发，降低恶意/损坏音频拖垮服务的风险。
- [ ] 扩展后台音乐列表：增加预设、置信度和分析状态列，按预设/状态筛选，支持单首分析、批量回填、重新分析、人工指定、清除人工覆盖和查看证据；所有写操作只允许管理员，读取继续遵守 M1 的实例访问策略。
- [ ] 暂定新增 `classification.enabled`、analyzer mode/endpoint/timeout/concurrency、`analyze_on_upload`、各预设阈值和权重等配置；落地时若修改 `config-example.yaml`，必须同步 README、环境变量、Compose 传递说明和配置测试。

### 注意事项与风险护栏

- [ ] 在引入任何二进制或模型前完成许可证评审并记录版本。当前调研中 [Essentia 的许可](https://essentia.upf.edu/licensing_information.html)及部分[预训练模型](https://essentia.upf.edu/models.html)可能涉及 AGPLv3、CC BY-NC-SA 或商业授权，与本项目 MIT 分发方式不能直接混为一谈；在结论明确前只保留适配器和本地实验，不把它们打进正式镜像。
- [ ] 模型标签通常能覆盖 D&B、Dubstep、Electro House、Trance、Progressive House、Synthwave、Hardstyle 等大类，但未必有稳定的 Complextro、Chillstep、Melodic Dubstep 精细标签；这些边界必须由别名、优先级和音频特征补足，不能把模型输出当真值。
- [ ] 建立可复现的本地 gold set：每个预设约 30–50 首人工确认曲目，并包含融合风格和明确不属于四类的负样本；版权音乐不提交到仓库或公开 CI，只提交标注清单、合法/合成短样本和可复现测试工具。
- [ ] 基准至少报告每类 precision/recall、macro-F1、高置信度 precision、自动覆盖率/弃权率、混淆矩阵、单曲 CPU 时间、峰值内存和镜像增量；在基准完成前不承诺具体准确率。
- [ ] 记录队列长度、各状态数量、分析耗时、失败率、规则/分析器版本；日志不得泄露宿主机路径、原始模型异常中的敏感内容或外部服务凭证。
- [ ] 所有分类结果都视为可修正派生数据：重跑不得覆盖人工选择，规则/模型升级可批量标记过期并回滚，删除音乐时同步清理任务、分析和分数记录。

验收标准：固定流派 fixture 能稳定验证别名、交叉风格优先级和弃权逻辑；人工覆盖在重新分析后仍生效且可清除；替换音频后旧分析会失效；服务重启、超时、损坏文件、分析器离线和重复入队不会阻塞上传/播放或产生并发重复结果；管理员可查看证据并批量处理低置信度曲目；预设筛选、权限、迁移与删除清理在 SQLite/PostgreSQL 上通过测试；最终入选分析器在本地 gold set 上有可复现的评估报告和明确的许可证结论。

## 保留但后置的安全债务

- [ ] 引入 refresh token 与可撤销会话：短期 access token、refresh token 轮换、服务端撤销、单设备/全部设备登出和安全存储；在此之前不把当前 `localStorage` access token 模型描述为长期会话方案。

该项继续保留，但不阻塞 M1 的元数据建模。实现时必须同时定义 Web 会话和兼容客户端凭证的边界，避免出现两套不可撤销的长期令牌。

## 明确不进入近期里程碑

- CSS Anchor Positioning、Popover/`dialog` 替换、频谱可视化、WebCodecs、WebGPU 等浏览器技术展示。
- 音频转码集群、DRM、商业流媒体接入、基于用户行为的通用推荐/协同过滤、社交动态、原生客户端、多租户 SaaS、Kubernetes 或多节点高可用；M5 的固定场景预设分类不属于这里的推荐系统。
- 完整 MusicBrainz 镜像、自动向 MusicBrainz 提交数据、首阶段回写源音频标签、无人确认的批量元数据覆盖。

## 实施约束

- 每个里程碑继续拆成可独立验证的原子提交；数据库迁移、API 契约、前端消费和测试应按依赖顺序提交。
- 前端只使用 pnpm 并优先通过根目录 Makefile；新增 UI 文案同时维护中英文 i18n。
- 不引入新的 UI 框架；继续使用 Element Plus、Pinia 和 Vue Router。
- 不删除 `cmd/server/dist/`；修改 `config-example.yaml` 时同步 README，新增环境变量时同时检查 Docker/Compose 文档。
- 行为变更必须覆盖 SQLite 和 PostgreSQL；外部服务通过 fixture 验证，本地播放路径保持可离线测试。
