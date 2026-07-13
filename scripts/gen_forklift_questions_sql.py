#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
解析叉车N1司机证理论考试题库 Markdown，生成 PostgreSQL 插入脚本。
- 按题目内容判断难度等级 (beginner/intermediate/advanced)
- 按主题分类到知识点 (knowledge_point)
- 适配 Docker 容器中的 PostgreSQL
用法: python gen_forklift_questions_sql.py
输出: .trae/documents/insert_forklift_questions.sql
"""

import re
import json
from pathlib import Path

PROJECT_ROOT = Path(__file__).resolve().parent.parent
INPUT_FILE = PROJECT_ROOT / ".workbuddy" / "叉车N1司机证理论考试题库.md"
OUTPUT_FILE = PROJECT_ROOT / ".trae" / "documents" / "insert_forklift_questions.sql"

# ============================================================
# 知识点定义（按主题分类，level 表示该知识点的难度层级）
# ============================================================
KNOWLEDGE_POINTS = [
    ("法律法规与证件管理", "beginner",
     "特种设备法律法规、N1证件管理、企业安全责任、报考与复审要求"),
    ("叉车结构与原理", "beginner",
     "叉车分类、组成结构、液压系统原理、性能参数、动力传递"),
    ("安全操作规范", "intermediate",
     "出车检查、行驶规范、限速要求、转弯会车、倒车停车、坡道行驶"),
    ("货物搬运安全", "intermediate",
     "起升操作、货物堆码、货叉使用、搬运禁忌、装载要求"),
    ("特殊环境作业", "intermediate",
     "防爆环境、恶劣天气、狭窄通道、电梯作业、涉水夜间作业"),
    ("检查与维护保养", "intermediate",
     "日常保养、内燃维护、电瓶维护、液压维护、制动轮胎维护、长期停放"),
    ("应急事故处理", "advanced",
     "侧翻处置、制动失灵、货物掉落、火灾灭火、液压灼伤、突发故障"),
    ("故障诊断基础", "advanced",
     "液压故障、制动故障、内燃故障、仪表报警、故障原因分析"),
    ("驾驶员职业要求", "beginner",
     "上岗条件、劳保用品、作业纪律、驾驶姿势、身体条件要求"),
]


def sql_escape(text: str) -> str:
    """PostgreSQL 字符串转义"""
    return text.replace("'", "''")


# ============================================================
# 难度判断（基于题目内容关键词分析）
# ============================================================
ADVANCED_KEYWORDS = [
    "故障", "失灵", "侧翻", "应急", "处置", "冒黑烟", "冒蓝烟", "冒白烟",
    "黑烟", "蓝烟", "白烟", "诊断", "排除", "会导致", "首选", "正确处置",
    "处理方法", "危害", "风险", "损坏", "变形", "漏油", "异响",
    "发软", "跑偏", "卡顿", "过热", "无力", "缓慢", "抖动", "间隙过大",
    "方向盘自由", "转向沉重", "制动踏板", "刹车油", "制动液",
    "液压油温", "油温过高", "起升无力", "自动下沉", "自然下降",
    "鼓包", "烧机油", "燃烧不充分", "动力不足", "空气滤清器",
    "离合结合", "内泄", "泄漏", "破裂", "爆裂", "灼伤",
    "紧急", "避险", "跳车", "致死", "事故",
]

BEGINNER_KEYWORDS = [
    "属于", "特种设备", "证书", "N1", "代号", "有效期", "4年", "复审",
    "3个月", "领证", "上岗证", "监管部门", "市场监管局", "市场监督管理",
    "特种设备作业", "年检", "1年", "报考", "年龄", "18周岁", "初中",
    "学历", "安全第一", "预防为主", "综合治理", "节能环保",
    "全国通用", "异地", "补办", "作废", "涂改", "伪造", "转借",
    "安全规章", "安全管理制度", "培训", "企业", "单位负责人",
    "责任", "ISO", "工业车辆", "平衡重式", "前移式", "插腿式",
    "侧面叉车", "额定起重量", "载荷中心", "帕斯卡",
    "严禁", "酒后", "无证", "载人", "超载",
]


def judge_difficulty(content: str, options: dict = None, q_type: str = "") -> str:
    """根据题目内容判断难度等级"""
    text = content
    if options:
        text += " " + " ".join(str(v) for v in options.values())

    # advanced: 涉及故障诊断、应急处理、原因分析
    for kw in ADVANCED_KEYWORDS:
        if kw in text:
            return "advanced"

    # beginner: 基础常识、证件管理、直接记忆
    for kw in BEGINNER_KEYWORDS:
        if kw in text:
            return "beginner"

    # 其余为 intermediate
    return "intermediate"


# ============================================================
# 知识点分类（按主题归类）
# ============================================================
def classify_knowledge_point(content: str, options: dict = None, q_type: str = "") -> str:
    """根据题目内容分类到对应知识点"""
    text = content
    if options:
        text += " " + " ".join(str(v) for v in options.values())

    # 1. 应急事故处理（最高优先级，关键词明确）
    emergency_kw = [
        "应急", "事故", "侧翻", "失灵", "火灾", "灭火", "灼伤", "碰撞",
        "掉落", "倒塌", "散落", "突发", "紧急", "避险", "跳车", "致死",
        "灭火器", "PASS", "119", "急救", "疏散", "保护现场", "上报",
        "制动失灵", "制动踏板发软", "刹车跑偏",
    ]
    for kw in emergency_kw:
        if kw in text:
            return "应急事故处理"

    # 2. 故障诊断基础
    fault_kw = [
        "故障", "冒黑烟", "冒蓝烟", "冒白烟", "黑烟", "蓝烟", "白烟",
        "漏油", "异响", "卡顿", "过热", "无力", "缓慢", "抖动",
        "间隙过大", "方向盘自由", "转向沉重", "发软", "跑偏",
        "液压油温", "油温过高", "起升无力", "自动下沉", "自然下降",
        "鼓包", "烧机油", "燃烧不充分", "动力不足", "空气滤清器",
        "内泄", "泄漏", "诊断", "排除", "原因", "会导致",
        "仪表", "指示灯", "报警", "红色指示灯",
    ]
    for kw in fault_kw:
        if kw in text:
            return "故障诊断基础"

    # 3. 法律法规与证件管理
    law_kw = [
        "证书", "N1", "代号", "有效期", "复审", "领证", "上岗证",
        "监管部门", "市场监管局", "市场监督管理", "特种设备安全法",
        "特种设备作业", "证件", "年检", "报考", "年龄", "18周岁", "初中",
        "学历", "安全第一", "预防为主", "综合治理", "节能环保",
        "全国通用", "异地", "补办", "作废", "涂改", "伪造", "转借",
        "安全规章", "安全管理制度", "培训", "企业", "单位负责人",
        "责任", "特种设备", "属于", "ISO", "工业车辆",
        "违章", "违规", "报废", "监管",
    ]
    for kw in law_kw:
        if kw in text:
            return "法律法规与证件管理"

    # 4. 驾驶员职业要求
    driver_kw = [
        "劳保", "安全帽", "安全鞋", "工作服", "手套", "反光背心",
        "拖鞋", "首饰", "着装", "酒后", "疲劳", "闲聊", "接电话", "打电话",
        "身体不适", "头晕", "乏力", "职业", "纪律", "驾驶姿势", "安全带",
        "穿戴", "佩戴",
    ]
    for kw in driver_kw:
        if kw in text:
            return "驾驶员职业要求"

    # 5. 特殊环境作业
    special_kw = [
        "易燃易爆", "防爆", "粉尘", "恶劣天气", "雨雪", "结冰",
        "暴雨", "大雾", "强风", "积水", "湿滑", "泥泞", "松软",
        "凹凸", "高温", "低温", "寒冷", "冬季", "预热", "防冻",
        "密闭", "化工", "油气", "夜间", "照明不良",
    ]
    for kw in special_kw:
        if kw in text:
            return "特殊环境作业"

    # 6. 检查与维护保养
    maintain_kw = [
        "保养", "维护", "维修", "检修", "更换", "充电", "蓄电池", "电池",
        "蒸馏水", "电解液", "液压油", "机油", "冷却液", "刹车油", "制动液",
        "润滑", "黄油", "链条", "轮胎", "气压", "花纹",
        "磨损", "锈蚀", "裂纹", "长期停放", "封存", "保管",
        "日常保养", "定期保养", "十字作业法", "空气滤清器", "滤芯",
        "怠速", "预热", "降温", "检查", "出车前",
    ]
    for kw in maintain_kw:
        if kw in text:
            return "检查与维护保养"

    # 7. 货物搬运安全
    cargo_kw = [
        "起升", "装卸", "堆码", "堆垛", "托盘", "货物", "搬运",
        "捆绑", "重心", "偏载", "单边叉", "货叉间距", "插入深度",
        "超高", "超宽", "超长", "惯性滑行", "顶推", "拖拽", "撞击",
        "高位", "货架", "码放", "倒塌", "滑落",
    ]
    for kw in cargo_kw:
        if kw in text:
            return "货物搬运安全"

    # 8. 叉车结构与原理
    struct_kw = [
        "结构", "组成", "原理", "帕斯卡", "液压泵", "液压阀", "液压缸",
        "齿轮泵", "叶片泵", "溢流阀", "安全阀", "单向阀", "节流阀", "换向阀",
        "门架", "油缸", "油箱", "滤油器", "平衡重", "额定起重量",
        "载荷中心", "起升高度", "转弯半径", "爬坡", "倾角", "前倾", "后倾",
        "自由起升", "动力传递", "液力变矩器", "传动轴", "变速箱",
        "驱动桥", "转向桥", "电气系统", "防护顶棚", "护顶架", "护额架",
        "倒车报警", "控制器", "电机",
    ]
    for kw in struct_kw:
        if kw in text:
            return "叉车结构与原理"

    # 9. 安全操作规范（默认）
    return "安全操作规范"


# ============================================================
# 题目解析
# ============================================================
def parse_true_false_questions(lines):
    """解析判断题。格式: N. 题目内容 √ 或 ×"""
    questions = []
    pattern = re.compile(r'^(\d+)\.\s*(.+?)([√×])\s*$')
    for line in lines:
        line = line.strip()
        m = pattern.match(line)
        if m:
            num = int(m.group(1))
            content = m.group(2).strip()
            mark = m.group(3)
            answer = 'TRUE' if mark == '√' else 'FALSE'
            options = {"A": "正确", "B": "错误"}
            difficulty = judge_difficulty(content, options, 'true_false')
            kp = classify_knowledge_point(content, options, 'true_false')
            questions.append({
                'num': num,
                'content': content,
                'answer': answer,
                'options': options,
                'difficulty': difficulty,
                'kp': kp,
            })
    return questions


def parse_single_choice_questions(lines):
    """解析单选题。格式: N. 题目 A.xxx **B.xxx** C.xxx D.xxx"""
    questions = []
    for line in lines:
        line = line.strip()
        if not line:
            continue
        m = re.match(r'^(\d+)[\.、]\s*(.+)', line)
        if not m:
            continue
        num = int(m.group(1))
        rest = m.group(2).strip()

        bold_matches = list(re.finditer(r'\*\*(.+?)\*\*', rest))
        correct_letters = []
        for bm in bold_matches:
            bold_text = bm.group(1)
            letter_m = re.match(r'^([A-D])\s*[\.．、]', bold_text)
            if letter_m:
                correct_letters.append(letter_m.group(1))

        clean_rest = re.sub(r'\*\*', '', rest)
        opt_pattern = re.compile(r'([A-D])\s*[\.．、]\s*')
        opt_matches = list(opt_pattern.finditer(clean_rest))

        if len(opt_matches) < 2:
            continue

        question_text = clean_rest[:opt_matches[0].start()].strip()
        options = {}
        for i, om in enumerate(opt_matches):
            letter = om.group(1)
            start = om.end()
            if i + 1 < len(opt_matches):
                end = opt_matches[i + 1].start()
            else:
                end = len(clean_rest)
            opt_text = clean_rest[start:end].strip()
            opt_text = re.sub(r'\s+', ' ', opt_text).strip()
            options[letter] = opt_text

        if not correct_letters:
            continue

        answer = correct_letters[0]
        difficulty = judge_difficulty(question_text, options, 'single_choice')
        kp = classify_knowledge_point(question_text, options, 'single_choice')
        questions.append({
            'num': num,
            'content': question_text,
            'options': options,
            'answer': answer,
            'difficulty': difficulty,
            'kp': kp,
        })
    return questions


def parse_multi_choice_questions(lines):
    """解析多选题。格式: N. 题目 **A.xxx B.xxx C.xxx** """
    questions = []
    for line in lines:
        line = line.strip()
        if not line:
            continue
        m = re.match(r'^(\d+)[\.、]\s*(.+)', line)
        if not m:
            continue
        num = int(m.group(1))
        rest = m.group(2).strip()

        bold_matches = list(re.finditer(r'\*\*(.+?)\*\*', rest))
        correct_letters = []
        for bm in bold_matches:
            bold_text = bm.group(1)
            letters = re.findall(r'([A-D])\s*[\.．、]', bold_text)
            correct_letters.extend(letters)

        clean_rest = re.sub(r'\*\*', '', rest)
        opt_pattern = re.compile(r'([A-D])\s*[\.．、]\s*')
        opt_matches = list(opt_pattern.finditer(clean_rest))

        if len(opt_matches) < 2:
            continue

        question_text = clean_rest[:opt_matches[0].start()].strip()
        options = {}
        for i, om in enumerate(opt_matches):
            letter = om.group(1)
            start = om.end()
            if i + 1 < len(opt_matches):
                end = opt_matches[i + 1].start()
            else:
                end = len(clean_rest)
            opt_text = clean_rest[start:end].strip()
            opt_text = re.sub(r'\s+', ' ', opt_text).strip()
            options[letter] = opt_text

        if not correct_letters:
            continue

        correct_letters = sorted(set(correct_letters))
        answer = ','.join(correct_letters)
        difficulty = judge_difficulty(question_text, options, 'multi_choice')
        kp = classify_knowledge_point(question_text, options, 'multi_choice')
        questions.append({
            'num': num,
            'content': question_text,
            'options': options,
            'answer': answer,
            'difficulty': difficulty,
            'kp': kp,
        })
    return questions


# ============================================================
# SQL 生成
# ============================================================
def generate_sql(tf_questions, sc_questions, mc_questions):
    """生成 SQL 插入脚本"""
    lines = []
    lines.append("-- ============================================================")
    lines.append("-- 叉车维修培训 - 题库插入脚本")
    lines.append("-- 题库来源: .workbuddy/叉车N1司机证理论考试题库.md")
    lines.append("-- 生成工具: scripts/gen_forklift_questions_sql.py")
    lines.append("-- 适配数据库: PostgreSQL (migrations/000001_init.up.sql)")
    lines.append("-- 执行环境: Docker 容器 forklift-pg-prod")
    lines.append("-- ============================================================")
    lines.append("")
    lines.append("BEGIN;")
    lines.append("")
    lines.append("-- ----------------------------------------------------------")
    lines.append("-- 安全清理：删除之前导入的题库数据（可选，重复执行时启用）")
    lines.append("-- 注意：如有考试答题记录引用这些题目，会级联删除相关记录")
    lines.append("-- 首次执行可注释掉以下 DELETE 语句")
    lines.append("-- ----------------------------------------------------------")
    lines.append("-- DELETE FROM question WHERE created_by_type = 'admin';")
    lines.append("-- DELETE FROM knowledge_point WHERE name IN (")
    for i, (name, _, _) in enumerate(KNOWLEDGE_POINTS):
        end = "," if i < len(KNOWLEDGE_POINTS) - 1 else ""
        lines.append(f"--     '{name}'{end}")
    lines.append("-- );")
    lines.append("")
    lines.append("-- ----------------------------------------------------------")
    lines.append("-- 1. 插入知识点（9个主题分类）")
    lines.append("-- ----------------------------------------------------------")
    lines.append("INSERT INTO knowledge_point (name, level, description, created_at) VALUES")
    for i, (name, level, desc) in enumerate(KNOWLEDGE_POINTS):
        end = "," if i < len(KNOWLEDGE_POINTS) - 1 else ";"
        lines.append(
            f"    ('{sql_escape(name)}', '{level}', '{sql_escape(desc)}', now()){end}"
        )
    lines.append("")

    # 统计各难度和知识点数量
    all_questions = tf_questions + sc_questions + mc_questions
    diff_count = {"beginner": 0, "intermediate": 0, "advanced": 0}
    kp_count = {name: 0 for name, _, _ in KNOWLEDGE_POINTS}
    for q in all_questions:
        diff_count[q['difficulty']] += 1
        kp_count[q['kp']] += 1

    lines.append("-- 知识点分布统计：")
    for name, _, _ in KNOWLEDGE_POINTS:
        lines.append(f"--   {name}: {kp_count[name]} 题")
    lines.append(f"-- 难度分布: beginner={diff_count['beginner']}, "
                 f"intermediate={diff_count['intermediate']}, "
                 f"advanced={diff_count['advanced']}")
    lines.append("")

    # 判断题
    lines.append("-- ===== 判断题 ({}题) =====".format(len(tf_questions)))
    lines.append("-- 分值: 2分/题 (与 examScoreMap 一致)")
    lines.append("INSERT INTO question (type, level, content, options, answer, explanation, score, knowledge_point_id, status, created_by_type, created_at, updated_at) VALUES")
    values = []
    for i, q in enumerate(tf_questions):
        options_json = json.dumps(q['options'], ensure_ascii=False)
        explanation = f"判断题第{q['num']}题：{'正确' if q['answer'] == 'TRUE' else '错误'}"
        vals = (
            f"('true_false', '{q['difficulty']}', "
            f"'{sql_escape(q['content'])}', "
            f"'{sql_escape(options_json)}'::jsonb, "
            f"'{q['answer']}', "
            f"'{sql_escape(explanation)}', "
            f"2, (SELECT id FROM knowledge_point WHERE name = '{sql_escape(q['kp'])}'), "
            f"'published', 'admin', now(), now())"
        )
        if i < len(tf_questions) - 1:
            vals += ","
        else:
            vals += ";"
        values.append(vals)
    lines.extend(values)
    lines.append("")

    # 单选题
    lines.append("-- ===== 单选题 ({}题) =====".format(len(sc_questions)))
    lines.append("-- 分值: 3分/题 (与 examScoreMap 一致)")
    lines.append("INSERT INTO question (type, level, content, options, answer, explanation, score, knowledge_point_id, status, created_by_type, created_at, updated_at) VALUES")
    values = []
    for i, q in enumerate(sc_questions):
        options_json = json.dumps(q['options'], ensure_ascii=False)
        explanation = f"单选题第{q['num']}题：正确答案{q['answer']}"
        vals = (
            f"('single_choice', '{q['difficulty']}', "
            f"'{sql_escape(q['content'])}', "
            f"'{sql_escape(options_json)}'::jsonb, "
            f"'{q['answer']}', "
            f"'{sql_escape(explanation)}', "
            f"3, (SELECT id FROM knowledge_point WHERE name = '{sql_escape(q['kp'])}'), "
            f"'published', 'admin', now(), now())"
        )
        if i < len(sc_questions) - 1:
            vals += ","
        else:
            vals += ";"
        values.append(vals)
    lines.extend(values)
    lines.append("")

    # 多选题
    lines.append("-- ===== 多选题 ({}题) =====".format(len(mc_questions)))
    lines.append("-- 分值: 4分/题 (与 examScoreMap 一致)")
    lines.append("INSERT INTO question (type, level, content, options, answer, explanation, score, knowledge_point_id, status, created_by_type, created_at, updated_at) VALUES")
    values = []
    for i, q in enumerate(mc_questions):
        options_json = json.dumps(q['options'], ensure_ascii=False)
        explanation = f"多选题第{q['num']}题：正确答案{q['answer']}"
        vals = (
            f"('multi_choice', '{q['difficulty']}', "
            f"'{sql_escape(q['content'])}', "
            f"'{sql_escape(options_json)}'::jsonb, "
            f"'{q['answer']}', "
            f"'{sql_escape(explanation)}', "
            f"4, (SELECT id FROM knowledge_point WHERE name = '{sql_escape(q['kp'])}'), "
            f"'published', 'admin', now(), now())"
        )
        if i < len(mc_questions) - 1:
            vals += ","
        else:
            vals += ";"
        values.append(vals)
    lines.extend(values)
    lines.append("")

    lines.append("-- ----------------------------------------------------------")
    lines.append("-- 验证查询")
    lines.append("-- ----------------------------------------------------------")
    lines.append("-- 按难度统计:")
    lines.append("-- SELECT level, count(*) FROM question WHERE created_by_type = 'admin' GROUP BY level ORDER BY level;")
    lines.append("")
    lines.append("-- 按知识点统计:")
    lines.append("-- SELECT kp.name, q.level, count(*) FROM question q")
    lines.append("-- JOIN knowledge_point kp ON q.knowledge_point_id = kp.id")
    lines.append("-- WHERE q.created_by_type = 'admin' GROUP BY kp.name, q.level ORDER BY kp.name;")
    lines.append("")
    lines.append("COMMIT;")
    lines.append("")
    total = len(tf_questions) + len(sc_questions) + len(mc_questions)
    lines.append("-- ============================================================")
    lines.append(f"-- 插入完成: 判断题 {len(tf_questions)} 题, 单选题 {len(sc_questions)} 题, 多选题 {len(mc_questions)} 题")
    lines.append(f"-- 合计: {total} 题")
    lines.append(f"-- 知识点: {len(KNOWLEDGE_POINTS)} 个")
    lines.append(f"-- 难度: beginner={diff_count['beginner']}, intermediate={diff_count['intermediate']}, advanced={diff_count['advanced']}")
    lines.append("-- ============================================================")
    lines.append("")

    return '\n'.join(lines)


def main():
    with open(INPUT_FILE, 'r', encoding='utf-8') as f:
        content = f.read()

    lines = content.split('\n')

    tf_start = tf_end = sc_start = sc_end = mc_start = mc_end = -1
    for i, line in enumerate(lines):
        if '第一部分' in line and '判断题' in line:
            tf_start = i + 1
        elif '第二部分' in line and '单选题' in line:
            tf_end = i
            sc_start = i + 1
        elif '第三部分' in line and '多选题' in line:
            sc_end = i
            mc_start = i + 1
    mc_end = len(lines)

    tf_questions = parse_true_false_questions(lines[tf_start:tf_end])
    sc_questions = parse_single_choice_questions(lines[sc_start:sc_end])
    mc_questions = parse_multi_choice_questions(lines[mc_start:mc_end])

    print(f"解析: 判断题 {len(tf_questions)} 题, 单选题 {len(sc_questions)} 题, 多选题 {len(mc_questions)} 题")
    total = len(tf_questions) + len(sc_questions) + len(mc_questions)
    print(f"合计: {total} 题")

    # 统计难度分布
    all_q = tf_questions + sc_questions + mc_questions
    diff_count = {"beginner": 0, "intermediate": 0, "advanced": 0}
    kp_count = {name: 0 for name, _, _ in KNOWLEDGE_POINTS}
    for q in all_q:
        diff_count[q['difficulty']] += 1
        kp_count[q['kp']] += 1
    print(f"\n难度分布: beginner={diff_count['beginner']}, intermediate={diff_count['intermediate']}, advanced={diff_count['advanced']}")
    print("\n知识点分布:")
    for name, _, _ in KNOWLEDGE_POINTS:
        print(f"  {name}: {kp_count[name]} 题")

    sql = generate_sql(tf_questions, sc_questions, mc_questions)
    OUTPUT_FILE.parent.mkdir(parents=True, exist_ok=True)
    with open(OUTPUT_FILE, 'w', encoding='utf-8') as f:
        f.write(sql)

    print(f"\nSQL 脚本已生成: {OUTPUT_FILE}")
    print(f"文件大小: {OUTPUT_FILE.stat().st_size / 1024:.1f} KB")


if __name__ == '__main__':
    main()
