from pathlib import Path

from reportlab.lib import colors
from reportlab.lib.enums import TA_CENTER, TA_LEFT
from reportlab.lib.pagesizes import A4
from reportlab.lib.styles import ParagraphStyle, getSampleStyleSheet
from reportlab.lib.units import mm
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.ttfonts import TTFont
from reportlab.platypus import (
    KeepTogether,
    Paragraph,
    SimpleDocTemplate,
    Spacer,
    Table,
    TableStyle,
)


ROOT = Path(__file__).resolve().parents[1]
OUTPUT = ROOT / "docs" / "Erain_AI开发补充版.pdf"


def register_fonts() -> tuple[str, str]:
    regular = Path(r"C:\Windows\Fonts\msyh.ttc")
    bold = Path(r"C:\Windows\Fonts\msyhbd.ttc")
    if not regular.exists() or not bold.exists():
        raise FileNotFoundError("未找到微软雅黑字体，请在 Windows 环境运行此脚本")
    pdfmetrics.registerFont(TTFont("MSYH", str(regular)))
    pdfmetrics.registerFont(TTFont("MSYH-Bold", str(bold)))
    return "MSYH", "MSYH-Bold"


FONT, FONT_BOLD = register_fonts()
BLUE = colors.HexColor("#1D4ED8")
NAVY = colors.HexColor("#111827")
GRAY = colors.HexColor("#4B5563")
LIGHT_GRAY = colors.HexColor("#E5E7EB")


styles = getSampleStyleSheet()
styles.add(
    ParagraphStyle(
        name="ResumeName",
        fontName=FONT_BOLD,
        fontSize=20,
        leading=24,
        textColor=NAVY,
        alignment=TA_CENTER,
        spaceAfter=2 * mm,
    )
)
styles.add(
    ParagraphStyle(
        name="ResumeTitle",
        fontName=FONT,
        fontSize=10.5,
        leading=14,
        textColor=GRAY,
        alignment=TA_CENTER,
        spaceAfter=1.5 * mm,
    )
)
styles.add(
    ParagraphStyle(
        name="ResumeLink",
        fontName=FONT,
        fontSize=8.5,
        leading=11,
        textColor=BLUE,
        alignment=TA_CENTER,
        spaceAfter=4 * mm,
    )
)
styles.add(
    ParagraphStyle(
        name="Section",
        fontName=FONT_BOLD,
        fontSize=12,
        leading=15,
        textColor=BLUE,
        borderColor=BLUE,
        borderWidth=0,
        borderPadding=0,
        spaceBefore=2.5 * mm,
        spaceAfter=1.5 * mm,
    )
)
styles.add(
    ParagraphStyle(
        name="Subsection",
        fontName=FONT_BOLD,
        fontSize=9.5,
        leading=13,
        textColor=NAVY,
        spaceBefore=1.5 * mm,
        spaceAfter=0.8 * mm,
    )
)
styles.add(
    ParagraphStyle(
        name="BodyCN",
        fontName=FONT,
        fontSize=8.2,
        leading=12.2,
        textColor=NAVY,
        alignment=TA_LEFT,
        spaceAfter=0.8 * mm,
    )
)
styles.add(
    ParagraphStyle(
        name="BulletCN",
        fontName=FONT,
        fontSize=8.1,
        leading=12,
        textColor=NAVY,
        leftIndent=3.5 * mm,
        firstLineIndent=-2.5 * mm,
        bulletIndent=0,
        spaceAfter=0.55 * mm,
    )
)
styles.add(
    ParagraphStyle(
        name="JobTitle",
        fontName=FONT_BOLD,
        fontSize=9.5,
        leading=13,
        textColor=NAVY,
    )
)
styles.add(
    ParagraphStyle(
        name="JobDate",
        fontName=FONT,
        fontSize=8.2,
        leading=12,
        textColor=GRAY,
        alignment=2,
    )
)


def p(text: str, style: str = "BodyCN") -> Paragraph:
    return Paragraph(text, styles[style])


def bullet(text: str) -> Paragraph:
    return Paragraph(f"• {text}", styles["BulletCN"])


def section(title: str):
    return [
        Spacer(1, 0.5 * mm),
        Table(
            [[Paragraph(title, styles["Section"])]],
            colWidths=[182 * mm],
            style=TableStyle(
                [
                    ("LINEBELOW", (0, 0), (-1, -1), 0.7, BLUE),
                    ("LEFTPADDING", (0, 0), (-1, -1), 0),
                    ("RIGHTPADDING", (0, 0), (-1, -1), 0),
                    ("TOPPADDING", (0, 0), (-1, -1), 0),
                    ("BOTTOMPADDING", (0, 0), (-1, -1), 1.5 * mm),
                ]
            ),
        ),
    ]


def job_header(name: str, date: str):
    return Table(
        [[Paragraph(name, styles["JobTitle"]), Paragraph(date, styles["JobDate"])]],
        colWidths=[137 * mm, 45 * mm],
        style=TableStyle(
            [
                ("VALIGN", (0, 0), (-1, -1), "TOP"),
                ("LEFTPADDING", (0, 0), (-1, -1), 0),
                ("RIGHTPADDING", (0, 0), (-1, -1), 0),
                ("TOPPADDING", (0, 0), (-1, -1), 0),
                ("BOTTOMPADDING", (0, 0), (-1, -1), 0.8 * mm),
            ]
        ),
    )


def add_page_number(canvas, doc):
    canvas.saveState()
    canvas.setFont(FONT, 7.5)
    canvas.setFillColor(colors.HexColor("#6B7280"))
    canvas.drawCentredString(A4[0] / 2, 8 * mm, f"Erain · 第 {doc.page} 页")
    canvas.restoreState()


story = [
    p("Erain", "ResumeName"),
    p("高级后端开发工程师｜PHP / Go｜高并发、微服务与 AI 协同研发", "ResumeTitle"),
    p(
        '<link href="https://github.com/Erain-byte/GRPC-GO-GATEWAY">GitHub：github.com/Erain-byte/GRPC-GO-GATEWAY</link>',
        "ResumeLink",
    ),
]

story += section("个人概要")
for item in [
    "10 年以上后端开发经验，长期参与业务系统需求分析、架构设计、核心开发、性能优化及线上稳定性治理。",
    "核心技术栈为 PHP、Swoole、MySQL、Redis 和 WebSocket，具备高并发接口、实时消息、订单及权限系统建设经验。",
    "具备 Go、Gin、gRPC、Protobuf 和 GORM 项目实践，能够参与 PHP 存量系统服务化拆分及 Go 服务建设。",
    "熟悉 MySQL 查询与索引优化、Redis 缓存治理、分布式锁、限流熔断、结构化日志和链路追踪。",
    "具备技术合伙人、技术组长及核心开发经历，能够推动需求拆解、技术方案评审、代码质量和研发流程落地。",
    "具备 AI 协同研发与独立全栈交付能力，熟练使用 Codex、ChatGPT 等工具完成需求拆解、方案推演、代码实现与重构、测试补充、故障定位及技术文档维护；建立“需求分析—任务拆分—AI 生成—人工审查—编译测试—联调部署”的研发闭环。",
]:
    story.append(bullet(item))

story += section("核心技能")
story.append(p("核心技术", "Subsection"))
for item in [
    "PHP：熟悉 PHP 运行机制、性能优化和工程化组织，长期使用 Yii / Yii2、ThinkPHP、Yaf 等框架开发业务系统。",
    "Swoole / EasySwoole：具备协程、连接池、异步任务和 WebSocket 长连接服务开发经验。",
    "MySQL：熟悉表结构设计、索引优化、慢查询及执行计划分析、事务控制和数据增长场景下的扩展方案。",
    "Redis：熟悉缓存更新、缓存预热、热点保护、穿透 / 击穿 / 雪崩治理、分布式锁及 Pub/Sub 应用。",
    "实时通信：具备 WebSocket、MQTT 和实时消息推送系统设计与开发经验。",
]:
    story.append(bullet(item))

story.append(p("Go 与微服务实践", "Subsection"))
for item in [
    "使用 Gin 构建 RESTful API，处理路由、中间件、参数校验、错误响应和接口版本管理。",
    "使用 gRPC + Protobuf 定义服务契约，实现超时控制、错误码映射、服务发现和客户端连接复用。",
    "使用 GORM 实现模型定义、数据访问、事务控制和复杂查询。",
    "熟悉 goroutine、channel、context、Mutex 等常用并发与生命周期控制机制。",
    "具备 API 网关、服务拆分、Consul 服务发现和 OpenTelemetry 链路追踪实践。",
]:
    story.append(bullet(item))

story.append(p("架构与工程实践", "Subsection"))
for item in [
    "能够设计 RESTful API 规范、统一响应结构、错误码体系、参数校验及接口文档规范。",
    "熟悉 JWT / OAuth2.0 认证方案及 RBAC / ABAC 权限模型。",
    "熟悉结构化日志、请求链路日志、日志采集和线上问题排查流程。",
    "熟悉限流、熔断、降级、重试等稳定性治理方案。",
    "具备服务拆分、缓存架构、数据库优化和高并发业务方案设计经验。",
]:
    story.append(bullet(item))

story.append(p("AI 协同研发与全栈交付", "Subsection"))
for item in [
    "熟练使用 Codex、ChatGPT 等 AI 编程工具，通过项目上下文、接口约束、验收标准和分阶段任务拆解，提高 Go、TypeScript、Vue 3、SQL、Docker 等技术栈的开发效率。",
    "能够使用 AI 辅助完成陌生代码库阅读、技术选型、Proto 与数据模型设计、代码生成与重构、故障分析、测试用例及部署文档编写。",
    "结合 Git Diff、编译检查、单元测试、集成联调、日志、OpenTelemetry、Jaeger 和 pprof 验证 AI 生成代码。",
    "具备 AI 辅助全栈实践，可独立完成 Go 微服务、Vue 3 + TypeScript 管理端、Expo Web 用户端及 Docker Compose 部署链路的端到端交付。",
    "理解 AI 输出的不确定性，重点审查并发安全、超时取消、错误处理、敏感配置、认证授权和资源释放等风险。",
]:
    story.append(bullet(item))

story += section("工作经历")

story.append(job_header("自由职业 / 独立开发", "2024.09 - 至今"))
for item in [
    "承接后端系统开发、存量项目维护、接口开发和性能优化等工作，根据项目规模独立完成需求分析、技术方案、开发交付及问题排查。",
    "参与 PHP 存量系统功能迭代与架构优化，处理接口、权限、订单、缓存及第三方系统对接等业务需求。",
    "持续进行 Go 技术栈实践，围绕 Gin、gRPC、Protobuf、Consul 和 Redis 搭建微服务及 API 网关项目。",
    "将 Codex、ChatGPT 纳入个人研发流程，围绕需求拆解、架构方案、代码实现、重构、测试、排障和文档开展 AI 协同开发。",
    "对 AI 生成结果进行人工代码审查，并通过编译测试、跨服务联调、链路追踪和实际服务器部署验证，处理 OAuth2 回调、mTLS 证书、服务注册、容器构建及前后端接口兼容等问题。",
]:
    story.append(bullet(item))

story.append(job_header("成都莫高云科技｜技术合伙人", "2022.04 - 2024.03"))
story.append(p("直播系统 / 实时互动系统", "Subsection"))
for item in [
    "负责直播业务后端架构设计、任务拆解、研发协作、代码评审及核心模块开发。",
    "设计 WebSocket 长连接服务，覆盖弹幕、在线人数、房间状态和实时通知等场景，支撑万级在线连接。",
    "使用 Gin 搭建业务 API，统一接入 JWT 鉴权、权限控制、参数校验、错误处理和结构化日志。",
    "使用 Redis 管理直播间状态、在线人数和热点数据，降低高频互动场景对数据库的访问压力。",
    "使用 goroutine 和 channel 处理异步消息分发及房间状态更新，改善实时消息处理效率。",
    "推动直播、用户和消息模块服务化拆分，并使用 gRPC 定义内部服务接口。",
]:
    story.append(bullet(item))

story.append(job_header("微元科技｜高级后端开发工程师", "2020.03 - 2021.06"))
story.append(p("Go 微服务 / API 网关 / 性能优化", "Subsection"))
for item in [
    "承担技术组长职能，参与 PHP 单体向 Go 微服务演进的方案评审、任务拆解、进度协同和代码质量管理。",
    "基于 Gin 搭建 API 网关，统一处理鉴权、限流、日志、链路追踪和错误响应。",
    "使用 gRPC + Protobuf 定义服务契约，规范服务间调用、错误处理、超时控制和接口兼容策略。",
    "使用 context 管理请求生命周期，实现超时控制、取消传播和跨服务调用链路治理。",
    "使用 goroutine 和 channel 实现异步任务处理及并发数据聚合，核心接口响应时间降低约 40%。",
    "接入 OpenTelemetry 链路追踪，改善跨服务慢请求和异常链路的定位效率。",
]:
    story.append(bullet(item))

story.append(p("数据库与缓存优化", "Subsection"))
for item in [
    "分析 MySQL 慢查询和执行计划，通过索引及查询结构优化，将重点查询耗时由 2 秒以上降低至 50 毫秒以内。",
    "参与分库分表方案设计，支持业务数据由百万级向更大规模增长。",
    "设计本地缓存与 Redis 多级缓存方案，热点接口压测 QPS 提升约 5 倍。",
    "实现缓存预热、更新及热点数据保护策略，降低缓存穿透、击穿和雪崩风险。",
    "使用 Redis 分布式锁处理高并发竞争场景，保障关键业务数据一致性。",
]:
    story.append(bullet(item))

story.append(job_header("广州万贝贸易｜高级 PHP 开发工程师", "2016.07 - 2019.11"))
story.append(p("Swoole 实时消息服务 / Yii2 业务系统", "Subsection"))
for item in [
    "基于 Swoole 搭建实时消息推送服务，支持万级 WebSocket 长连接和实时通知场景。",
    "使用协程池管理数据库及 Redis 连接，降低阻塞 IO 带来的资源占用。",
    "设计请求限流和熔断机制，提升高流量场景下的服务稳定性。",
    "基于 Yii2 开发企业后台管理系统，设计 RBAC 权限模型，支持菜单、接口和按钮级权限控制。",
    "构建电商订单系统，处理并发下单、库存校验、订单状态流转和支付回调。",
    "设计统一 API 基础能力，集成鉴权、参数校验、异常处理、日志和响应格式规范。",
]:
    story.append(bullet(item))

story.append(job_header("广州南鸥科技｜核心开发工程师", "2013.04 - 2016.06"))
story.append(p("多店管理系统 / 多租户业务系统", "Subsection"))
for item in [
    "基于 Yii 开发多店管理系统，实现门店管理、商品同步、订单分发和权限分级等核心功能。",
    "设计多租户数据及权限隔离方案，支持门店权限控制和多角色协作。",
    "优化数据库查询和业务接口性能，支撑多门店并发访问及订单处理。",
]:
    story.append(bullet(item))

story += section("代表项目")
story.append(p("AI 协同微服务与全栈交付实践", "Subsection"))
story.append(
    p(
        "技术栈：Go、Gin、gRPC、Protobuf、PostgreSQL、Redis、RabbitMQ、Consul、Keycloak、OpenTelemetry、Jaeger、Vue 3、TypeScript、Expo Web、Docker Compose、Codex"
    )
)
for item in [
    "使用 AI 协同方式完成 Gateway、Admin、User、Web3 等服务的需求拆解、接口设计、代码实现、重构及联调，构建 Gateway → User → Web3 跨服务调用链路。",
    "完成 Consul 服务注册与发现、Keycloak OAuth2/OIDC、内部 gRPC mTLS、限流、统一错误处理、Request ID、结构化日志及 OpenTelemetry 链路追踪等工程能力。",
    "使用 Vue 3 + TypeScript 构建管理端，使用 Expo Web 构建用户端，完成前端鉴权、API 调用、静态构建与 Nginx 容器部署。",
    "使用 Docker Compose 完成 PostgreSQL、Redis、RabbitMQ、Consul、Keycloak、Jaeger、Go 服务和前端应用的服务器部署与联调。",
    "通过人工审查、Go 编译测试、接口联调、Consul 健康检查、Jaeger Trace 和 pprof 验证 AI 生成结果。",
]:
    story.append(bullet(item))

story.append(p("Go 微服务与 API 网关实践", "Subsection"))
story.append(p("技术栈：Go、Gin、gRPC、Protobuf、Consul、Redis、OpenTelemetry"))
for item in [
    "设计 API 网关层，统一承载鉴权、日志、错误处理、超时控制和响应格式。",
    "定义 gRPC 服务契约，实现 HTTP 与 gRPC 错误码映射及请求上下文传播。",
    "接入 Consul 服务注册与发现，实现客户端连接复用及 gRPC Round-Robin 负载均衡。",
    "设计结构化日志和链路追踪方案，提升跨服务调用问题的定位效率。",
]:
    story.append(bullet(item))

story.append(p("实时消息推送系统", "Subsection"))
story.append(p("技术栈：PHP、EasySwoole、Swoole、Redis、MySQL、MQTT、WebSocket"))
for item in [
    "设计 WebSocket 长连接服务，支持连接管理、心跳检测、断线重连和实时消息推送。",
    "优化协程池与连接池，提高高并发场景下的资源利用率。",
    "使用 MQTT 及 Redis Pub/Sub 实现设备、服务和多进程之间的异步消息分发。",
    "设计限流、熔断及异常重试策略，支撑万级并发连接场景。",
]:
    story.append(bullet(item))

story += section("教育背景")
story.append(job_header("湖南大学｜计算机科学与技术｜本科", ""))


doc = SimpleDocTemplate(
    str(OUTPUT),
    pagesize=A4,
    rightMargin=14 * mm,
    leftMargin=14 * mm,
    topMargin=12 * mm,
    bottomMargin=14 * mm,
    title="Erain - 高级后端开发工程师 - AI 开发能力补充版",
    author="Erain",
)
doc.build(story, onFirstPage=add_page_number, onLaterPages=add_page_number)
print(OUTPUT)
