# Music Online Roadmap

## 产品主旨

Music Online 是一个面向个人、家庭或小团队的 **self-hosted 小型音乐平台**：用一个实例管理、检索和播放自己的音乐库，并兼容常见的 MusicBrainz Picard 标签。

- **Self-hosted**：单机、单容器或 Compose 即可部署；SQLite 是默认路径，PostgreSQL 是可选路径；备份、迁移和升级必须可控。
- **MusicBrainz 标签兼容**：Picard 标记过的文件在“浏览器解析 → API → 数据库 → 查看/编辑”链路中不应丢失常用标签和稳定 ID。
- **小型平台**：优先做好音乐库、播放、导入和少量用户协作，不向商业流媒体、社交网络或多节点媒体集群扩张。
- **离线优先**：本地音乐的导入和播放不能依赖 MusicBrainz 在线服务；在线查询只负责可选的补全和校对。

第一阶段的“标签兼容”不包含回写或修改源音频文件，也不代表完整镜像 MusicBrainz 数据库。标签基线以 [Picard Tag Mapping](https://picard-docs.musicbrainz.org/en/latest/appendices/tag_mapping.html) 为准。

现有的上传、播放、批量导入、重复检测、用户管理、TOTP、Docker/Compose、备份恢复、健康检查和 PWA 等能力已经完成，不再在 TODO 中保留勾选历史；完成记录以 Git 历史和 README 为准。

## M1：收口访问边界与元数据主干

这是下一阶段的发布阻断项。先让现有数据和接口表达同一件事，再增加在线匹配或更多页面。

- [ ] 增加实例访问策略：至少支持“登录后访问”和“公开只读”两种音乐库模式，并让列表、详情、音频流、封面、用户音乐和标签查询遵守同一策略。
- [ ] 增加注册策略：允许管理员关闭公开注册；关闭后由管理员创建用户或发放一次性邀请；配置文件、环境变量、Compose 示例和 README 必须同步说明，新实例仍可通过 `admin.bootstrap` 完成初始化。
- [ ] 处理当前公开的 `POST /api/v1/track/submit`：若保留 MusicBee 客户端兼容能力，改用可撤销且有写入范围的凭证并补契约文档；没有真实客户端契约时删除该公开写入口，不能继续匿名写库。
- [ ] 定义唯一的曲目元数据模型，打通当前彼此独立的 `vinyl` 与 `music_tags` 数据；可以合并模型，也可以建立明确关联，但不能继续让上传音乐和标签库各自演进。
- [ ] 明确并持久化 Picard 常用字段：标题、艺术家列表、专辑、专辑艺术家列表、音轨号/总数、碟号/总数、发行日期、原始发行日期、流派、注释和 ISRC。
- [ ] 明确并持久化 MusicBrainz ID 的语义和多值能力：`recordingid`、`trackid`（release track）、`albumid`（release）、`releasegroupid`、`artistid[]`、`albumartistid[]`；迁移掉含义模糊的通用 `musicbrainz_id` 命名。
- [ ] 修正标签匹配规则：稳定实体 ID 优先于文本；艺术家 ID 不能单独判定一首录音；“艺术家 + 标题”只能作为候选；展示文本保留原始大小写，规范化值只用于检索。
- [ ] 让单曲和批量导入完整传递上述字段。`music-metadata` 已能解析这些 MusicBrainz 字段，当前 `MusicMetadataFields` 和 `extractMetaFields()` 会把它们丢弃。
- [ ] 在曲目详情和编辑页展示、校验并编辑兼容字段；无 MusicBrainz 标签的普通文件必须继续正常工作。
- [ ] 为旧 SQLite/PostgreSQL 数据编写版本化迁移，并决定旧 `music_tags`、`use_count`、模糊匹配及兼容端点中哪些迁移、重构或删除，不留下第二套孤立元数据。

验收标准：准备 Picard 标记的 MP3、FLAC、M4A 固定样本，验证多种标签容器经过导入、持久化、查询和编辑后语义不丢失；验证旧数据库可升级；验证匿名请求不能越过实例访问策略或写入标签。

## M2：可选的 MusicBrainz 在线补全

- [ ] 实现真正的 MusicBrainz Web Service 客户端，通过 recording/release/artist ID 查找，并支持按现有标题、艺术家、专辑搜索候选；当前 `/mbid/lookup` 只查本地表，不能称为在线补全。
- [ ] 所有候选结果先展示差异并由用户确认，默认只补空值，不静默覆盖本地修改；批量操作必须能暂停、取消并逐项报告失败。
- [ ] 遵守 [MusicBrainz API](https://musicbrainz.org/doc/MusicBrainz_API) 约束：有联系方式的 User-Agent、全实例每秒不超过一次请求、超时、退避和缓存；MusicBrainz 不可用时本地功能正常降级。
- [ ] 为服务开关、API 基地址、User-Agent 联系方式、超时和缓存期限提供配置；所有新增配置同时出现在示例 YAML、环境变量文档、Compose 传递说明和配置测试中。
- [ ] 可选接入 Cover Art Archive，候选封面必须由用户确认并复制到受管理的上传目录；远程 URL 不直接成为永久媒体路径。
- [ ] 用固定响应 fixture 测试解析和匹配，不在普通单元测试或 CI 中实时依赖 MusicBrainz。

## M3：面向自托管音乐库的导入生命周期

- [ ] 增加服务端目录导入：管理员配置一个或多个只读导入根目录，手动触发扫描，首版只复制到受管理的上传目录，不修改源文件，也不引入常驻文件监听器。
- [ ] 扫描任务提供预览、进度、取消、逐文件错误和结束摘要；依靠文件哈希与元数据候选保证重复执行幂等。
- [ ] 处理源文件消失、同内容改名、标签变化和已有数据库记录的协调策略；任何自动删除或覆盖都必须先预览确认。
- [ ] 对导入根目录、符号链接和目标路径做边界校验；Docker 文档给出显式只读挂载示例，未配置的宿主机路径不可访问。
- [ ] 继续复用 M1 的同一套解析、校验、重复检测和 MusicBrainz 字段，不为浏览器上传与服务端扫描维护两条不同的元数据规则。

## M4：让元数据真正服务于浏览和播放

- [ ] 增加艺术家和专辑视图；有 MusicBrainz ID 时用稳定 ID 分组，没有时使用规范化文本回退，并允许播放整张专辑或加入队列。
- [ ] 扩充服务端筛选和 URL 状态，至少覆盖专辑、专辑艺术家、流派和发行年份；分页与访问策略保持一致。
- [ ] 增加用户播放列表：首版只做私有列表、排序、增删曲目和加入播放队列，不引入协作编辑、推荐算法或社交动态。
- [ ] 为无封面、缺标签、同名艺术家/专辑和多碟专辑提供明确的前端状态，并同步中英文 i18n。

## 保留但后置的安全债务

- [ ] 引入 refresh token 与可撤销会话：短期 access token、refresh token 轮换、服务端撤销、单设备/全部设备登出和安全存储；在此之前不把当前 `localStorage` access token 模型描述为长期会话方案。

该项继续保留，但不阻塞 M1 的元数据建模。实现时必须同时定义 Web 会话和兼容客户端凭证的边界，避免出现两套不可撤销的长期令牌。

## 明确不进入近期里程碑

- CSS Anchor Positioning、Popover/`dialog` 替换、频谱可视化、WebCodecs、WebGPU 等浏览器技术展示。
- 音频转码集群、DRM、商业流媒体接入、推荐算法、社交动态、原生客户端、多租户 SaaS、Kubernetes 或多节点高可用。
- 完整 MusicBrainz 镜像、自动向 MusicBrainz 提交数据、首阶段回写源音频标签、无人确认的批量元数据覆盖。

## 实施约束

- 每个里程碑继续拆成可独立验证的原子提交；数据库迁移、API 契约、前端消费和测试应按依赖顺序提交。
- 前端只使用 pnpm 并优先通过根目录 Makefile；新增 UI 文案同时维护中英文 i18n。
- 不引入新的 UI 框架；继续使用 Element Plus、Pinia 和 Vue Router。
- 不删除 `cmd/server/dist/`；修改 `config-example.yaml` 时同步 README，新增环境变量时同时检查 Docker/Compose 文档。
- 行为变更必须覆盖 SQLite 和 PostgreSQL；外部服务通过 fixture 验证，本地播放路径保持可离线测试。
