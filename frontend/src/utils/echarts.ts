/**
 * ECharts 按需引入的唯一入口（方案 11.9）
 *
 * 原先每个用到图表的地方都 `import * as echarts from 'echarts'`，会把全部图表类型
 * （地图 / 关系图 / 3D / 词云…）与两种渲染器一起打进 bundle，而项目实际只用 4 种图。
 *
 * 收敛成一处 `use()` 注册的好处：
 * 1. 调用方只 `import echarts from '@/utils/echarts'`，不用关心注册了什么；
 * 2. 将来要加一种图（比如折线改面积图），只改这里一行，不会有人忘记注册而
 *    得到一个「画不出来但不报错」的空图表 —— 这是按需引入最典型的踩坑点。
 *
 * 已核对全仓 option，实际用到的只有下面这些：
 * - series: bar / line / pie / radar
 * - 组件: grid（含 xAxis / yAxis）、tooltip、legend、radar、title
 * - axisPointer: tooltip 配 `axisPointer: { type: 'shadow' }` 时必须显式注册，
 *   全量包自带，按需包不带，漏了会静默丢失十字准线/阴影指示器。
 */
import * as echarts from 'echarts/core'
import { BarChart, LineChart, PieChart, RadarChart } from 'echarts/charts'
import {
  AxisPointerComponent,
  GridComponent,
  LegendComponent,
  RadarComponent,
  TitleComponent,
  TooltipComponent
} from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'

echarts.use([
  BarChart,
  LineChart,
  PieChart,
  RadarChart,
  GridComponent,
  TooltipComponent,
  LegendComponent,
  RadarComponent,
  TitleComponent,
  AxisPointerComponent,
  CanvasRenderer
])

export default echarts
export type { ECharts, EChartsCoreOption } from 'echarts/core'
