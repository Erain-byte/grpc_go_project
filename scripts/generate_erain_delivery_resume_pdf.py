from pathlib import Path

from reportlab.lib import colors
from reportlab.lib.enums import TA_CENTER
from reportlab.lib.pagesizes import A4
from reportlab.lib.styles import ParagraphStyle, getSampleStyleSheet
from reportlab.lib.units import mm
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.ttfonts import TTFont
from reportlab.platypus import PageBreak, Paragraph, SimpleDocTemplate, Spacer, Table, TableStyle


ROOT = Path(__file__).resolve().parents[1]
OUTPUT = ROOT / "docs" / "Erain_Go_AI全栈投递版.pdf"

pdfmetrics.registerFont(TTFont("MSYH", r"C:\Windows\Fonts\msyh.ttc"))
pdfmetrics.registerFont(TTFont("MSYH-Bold", r"C:\Windows\Fonts\msyhbd.ttc"))

FONT = "MSYH"
BOLD = "MSYH-Bold"
NAVY = colors.HexColor("#111827")
BLUE = colors.HexColor("#1D4ED8")
GRAY = colors.HexColor("#4B5563")
PALE_BLUE = colors.HexColor("#EFF6FF")
LINE = colors.HexColor("#CBD5E1")

styles = getSampleStyleSheet()
styles.add(ParagraphStyle(name="NameCN", fontName=BOLD, fontSize=20, leading=23, textColor=NAVY, alignment=TA_CENTER, spaceAfter=1.2 * mm))
styles.add(ParagraphStyle(name="TitleCN", fontName=BOLD, fontSize=11, leading=15, textColor=BLUE, alignment=TA_CENTER, spaceAfter=1.2 * mm))
styles.add(ParagraphStyle(name="MetaCN", fontName=FONT, fontSize=8.2, leading=11, textColor=GRAY, alignment=TA_CENTER, spaceAfter=3 * mm))
styles.add(ParagraphStyle(name="SectionCN", fontName=BOLD, fontSize=11, leading=14, textColor=BLUE, spaceBefore=2.2 * mm, spaceAfter=1.2 * mm))
styles.add(ParagraphStyle(name="BodyCN2", fontName=FONT, fontSize=8.25, leading=11.7, textColor=NAVY, spaceAfter=0.5 * mm))
styles.add(ParagraphStyle(name="BulletCN2", fontName=FONT, fontSize=8.15, leading=11.6, textColor=NAVY, leftIndent=3.5 * mm, firstLineIndent=-2.5 * mm, spaceAfter=0.45 * mm))
styles.add(ParagraphStyle(name="JobCN", fontName=BOLD, fontSize=9.4, leading=12.5, textColor=NAVY))
styles.add(ParagraphStyle(name="DateCN", fontName=FONT, fontSize=8.1, leading=11.5, textColor=GRAY, alignment=2))
styles.add(ParagraphStyle(name="SmallCN", fontName=FONT, fontSize=7.8, leading=11, textColor=NAVY))
styles.add(ParagraphStyle(name="SmallBoldCN", fontName=BOLD, fontSize=8, leading=11, textColor=NAVY))
styles.add(ParagraphStyle(name="MetricCN", fontName=BOLD, fontSize=9.1, leading=12, textColor=BLUE, alignment=TA_CENTER))
styles.add(ParagraphStyle(name="MetricNoteCN", fontName=FONT, fontSize=7.3, leading=10, textColor=GRAY, alignment=TA_CENTER))


def para(text: str, style: str = "BodyCN2") -> Paragraph:
    return Paragraph(text, styles[style])


def bullet(text: str) -> Paragraph:
    return Paragraph(f"• {text}", styles["BulletCN2"])


def section(title: str):
    return Table(
        [[Paragraph(title, styles["SectionCN"])]],
        colWidths=[182 * mm],
        style=TableStyle(
            [
                ("LINEBELOW", (0, 0), (-1, -1), 0.7, BLUE),
                ("LEFTPADDING", (0, 0), (-1, -1), 0),
                ("RIGHTPADDING", (0, 0), (-1, -1), 0),
                ("TOPPADDING", (0, 0), (-1, -1), 0),
                ("BOTTOMPADDING", (0, 0), (-1, -1), 1.2 * mm),
            ]
        ),
    )


def job(name: str, date: str):
    return Table(
        [[para(name, "JobCN"), para(date, "DateCN")]],
        colWidths=[137 * mm, 45 * mm],
        style=TableStyle(
            [
                ("VALIGN", (0, 0), (-1, -1), "TOP"),
                ("LEFTPADDING", (0, 0), (-1, -1), 0),
                ("RIGHTPADDING", (0, 0), (-1, -1), 0),
                ("TOPPADDING", (0, 0), (-1, -1), 1.2 * mm),
                ("BOTTOMPADDING", (0, 0), (-1, -1), 0.5 * mm),
            ]
        ),
    )


def footer(canvas, doc):
    canvas.saveState()
    canvas.setFont(FONT, 7.3)
    canvas.setFillColor(colors.HexColor("#64748B"))
    canvas.drawCentredString(A4[0] / 2, 7.5 * mm, f"Erain · Go / PHP / AI 全栈交付 · 第 {doc.page} 页")
    canvas.restoreState()


story = [
    para("Erain", "NameCN"),
    para("高级 Go / PHP 后端工程师｜微服务架构｜AI 全栈交付", "TitleCN"),
    para(
        '10 年以上后端经验｜技术合伙人 / 技术组长经历｜GitHub：<link href="https://github.com/Erain-byte/grpc_go_project">github.com/Erain-byte/grpc_go_project</link>',
        "MetaCN",
    ),
    section("职业概述"),
]

for text in [
    "10 年以上后端研发经验，覆盖需求分析、架构设计、核心开发、性能优化、线上治理和团队协作；具备技术合伙人、技术组长及核心开发经历。",
    "具备 PHP → Go 服务化演进实践，使用 Gin、gRPC、Protobuf、Consul、Redis、OpenTelemetry 构建 API 网关与微服务调用链路。",
    "具备直播与实时互动系统经验，设计 WebSocket 长连接、房间状态、在线人数、弹幕及实时通知等能力，支撑万级在线连接。",
    "深度使用 Codex、ChatGPT 开展 AI 协同研发，能够独立完成 Go 后端、Vue 3 + TypeScript 管理端、Expo Web 用户端及 Docker 部署的端到端交付。",
    "擅长 MySQL 与 Redis 性能治理、高并发业务设计、服务稳定性建设及复杂线上问题定位。",
]:
    story.append(bullet(text))

story.append(section("岗位匹配能力"))
skill_rows = [
    [para("Go / 微服务", "SmallBoldCN"), para("Gin、gRPC、Protobuf、GORM、goroutine、channel、context、Mutex、API Gateway、Consul", "SmallCN")],
    [para("PHP / 常驻进程", "SmallBoldCN"), para("PHP、Yii / Yii2、ThinkPHP、Yaf、Swoole、EasySwoole、协程、连接池、WebSocket", "SmallCN")],
    [para("数据与缓存", "SmallBoldCN"), para("MySQL 索引与 EXPLAIN、事务、分库分表；Redis 多级缓存、热点保护、分布式锁、Pub/Sub", "SmallCN")],
    [para("稳定性与观测", "SmallBoldCN"), para("限流、熔断、降级、重试、结构化日志、Request ID、OpenTelemetry、Jaeger、pprof", "SmallCN")],
    [para("AI 开发", "SmallBoldCN"), para("Codex、ChatGPT；需求拆解、方案推演、代码生成与重构、人工审查、测试、排障及文档", "SmallCN")],
    [para("全栈与交付", "SmallBoldCN"), para("Vue 3、TypeScript、Expo Web、OAuth2/OIDC、Keycloak、mTLS、Docker Compose、Nginx、Linux", "SmallCN")],
]
story.append(
    Table(
        skill_rows,
        colWidths=[29 * mm, 153 * mm],
        style=TableStyle(
            [
                ("BACKGROUND", (0, 0), (0, -1), PALE_BLUE),
                ("GRID", (0, 0), (-1, -1), 0.35, LINE),
                ("VALIGN", (0, 0), (-1, -1), "MIDDLE"),
                ("LEFTPADDING", (0, 0), (-1, -1), 2 * mm),
                ("RIGHTPADDING", (0, 0), (-1, -1), 2 * mm),
                ("TOPPADDING", (0, 0), (-1, -1), 1.1 * mm),
                ("BOTTOMPADDING", (0, 0), (-1, -1), 1.1 * mm),
            ]
        ),
    )
)

story.append(section("可量化成果"))
metrics = [
    [para("2s+ → 50ms", "MetricCN"), para("QPS ≈ 5 倍", "MetricCN"), para("响应提升 ≈ 40%", "MetricCN"), para("万级连接", "MetricCN")],
    [para("MySQL 重点查询优化", "MetricNoteCN"), para("Redis 多级缓存压测", "MetricNoteCN"), para("并发聚合核心接口", "MetricNoteCN"), para("WebSocket 实时系统", "MetricNoteCN")],
]
story.append(
    Table(
        metrics,
        colWidths=[45.5 * mm] * 4,
        style=TableStyle(
            [
                ("BOX", (0, 0), (-1, -1), 0.5, LINE),
                ("INNERGRID", (0, 0), (-1, -1), 0.35, LINE),
                ("BACKGROUND", (0, 0), (-1, -1), colors.HexColor("#F8FAFC")),
                ("TOPPADDING", (0, 0), (-1, 0), 1.7 * mm),
                ("BOTTOMPADDING", (0, 1), (-1, 1), 1.7 * mm),
            ]
        ),
    )
)

story.append(section("近期工作经历"))
story.append(job("自由职业 / 独立开发", "2024.09 - 至今"))
for text in [
    "独立承担后端系统开发、存量项目维护、接口建设和性能优化，完成需求分析、技术方案、开发交付、服务器部署及问题排查。",
    "围绕 Gin、gRPC、Protobuf、Consul、Redis 和 PostgreSQL 建设 Go 微服务及 API 网关项目。",
    "将 Codex、ChatGPT 纳入研发流程，完成需求拆解、架构方案、代码实现、重构、测试、排障和文档，并对生成代码进行人工审查与联调验证。",
    "实际处理 OAuth2/OIDC 回调、Keycloak、mTLS、Consul 注册发现、容器构建、跨服务调用及前后端接口兼容问题。",
]:
    story.append(bullet(text))

story.append(job("成都莫高云科技｜技术合伙人", "2022.04 - 2024.03"))
for text in [
    "负责直播业务后端架构设计、需求拆解、研发协作、代码评审及核心模块开发。",
    "设计 WebSocket 长连接服务，覆盖弹幕、在线人数、房间状态和实时通知，支撑万级在线连接。",
    "使用 Gin、JWT、Redis、goroutine 和 channel 建设业务 API、热点缓存及异步消息处理链路。",
    "推动直播、用户和消息模块服务化拆分，并使用 gRPC 定义内部服务接口。",
]:
    story.append(bullet(text))

story.append(PageBreak())

story.append(section("工作经历（续）"))
story.append(job("微元科技｜高级后端开发工程师 / 技术组长", "2020.03 - 2021.06"))
for text in [
    "参与 PHP 单体向 Go 微服务演进的方案评审、服务拆分、任务协同和代码质量管理。",
    "基于 Gin 搭建 API 网关，统一处理鉴权、限流、日志、链路追踪和错误响应；使用 gRPC + Protobuf 规范内部服务契约。",
    "使用 context 管理超时、取消传播和请求生命周期；使用 goroutine、channel 完成异步处理与并发数据聚合，核心接口响应时间降低约 40%。",
    "通过索引及查询结构优化，将重点 MySQL 查询耗时由 2 秒以上降低至 50 毫秒以内；Redis 多级缓存压测 QPS 提升约 5 倍。",
    "接入 OpenTelemetry 追踪跨服务慢请求与异常调用链路，提升线上定位效率。",
]:
    story.append(bullet(text))

story.append(job("广州万贝贸易｜高级 PHP 开发工程师", "2016.07 - 2019.11"))
for text in [
    "基于 Swoole 搭建实时消息推送服务，使用协程池管理数据库及 Redis 连接，支持万级 WebSocket 长连接。",
    "基于 Yii2 开发企业后台和 RBAC 权限体系，完成菜单、接口、按钮级权限控制。",
    "建设电商订单系统，处理并发下单、库存校验、订单状态流转和支付回调，并完善限流、熔断和统一 API 能力。",
]:
    story.append(bullet(text))

story.append(job("广州南鸥科技｜核心开发工程师", "2013.04 - 2016.06"))
for text in [
    "基于 Yii 开发多店、多租户业务系统，实现门店、商品、订单、权限分级及多角色协作。",
    "负责数据与权限隔离、数据库查询优化和业务接口性能治理，支撑多门店并发访问与订单处理。",
]:
    story.append(bullet(text))

story.append(section("代表项目"))
story.append(para("AI 协同 Go 微服务与全栈交付平台", "JobCN"))
story.append(para("Go、Gin、gRPC、Protobuf、PostgreSQL、Redis、RabbitMQ、Consul、Keycloak、OpenTelemetry、Jaeger、Vue 3、TypeScript、Expo Web、Docker Compose、Codex", "SmallCN"))
for text in [
    "完成 Gateway、Admin、User、Web3 多服务的接口设计、代码实现与联调，构建 Gateway → User → Web3 跨服务调用链路。",
    "落地 Consul 注册发现、Keycloak OAuth2/OIDC、内部 gRPC mTLS、限流、统一错误、Request ID、结构化日志及 OpenTelemetry 追踪。",
    "完成 Vue 3 管理端、Expo Web 用户端以及 PostgreSQL、Redis、RabbitMQ、Consul、Keycloak、Jaeger 与 Go 服务的 Docker Compose 部署。",
    "采用“任务拆解—AI 生成—人工审查—编译测试—接口联调—部署验证”流程，保证 AI 代码可运行、可观测和可维护。",
]:
    story.append(bullet(text))

story.append(para("直播与实时消息系统", "JobCN"))
story.append(para("PHP、Swoole、EasySwoole、Go、Redis、MySQL、MQTT、WebSocket", "SmallCN"))
for text in [
    "设计长连接管理、心跳检测、断线重连、房间状态、在线人数、弹幕和实时通知链路。",
    "优化协程池与连接池，使用 MQTT、Redis Pub/Sub 和异步任务提升高并发场景的资源利用率与消息处理能力。",
]:
    story.append(bullet(text))

story.append(section("教育背景"))
story.append(job("湖南大学｜计算机科学与技术｜本科", ""))

doc = SimpleDocTemplate(
    str(OUTPUT),
    pagesize=A4,
    leftMargin=14 * mm,
    rightMargin=14 * mm,
    topMargin=10 * mm,
    bottomMargin=13 * mm,
    title="Erain - 高级 Go PHP 后端工程师 - AI 全栈投递版",
    author="Erain",
)
doc.build(story, onFirstPage=footer, onLaterPages=footer)
print(OUTPUT)
