import * as THREE from 'three'
import { CSS2DObject } from 'three/examples/jsm/renderers/CSS2DRenderer.js'

const PARTS_CONFIG = [
  {
    partId: 'body',
    name: '车身',
    info: '叉车主体结构，承载所有核心组件。日常需检查车身有无变形、裂纹和腐蚀。',
    maintenance: ['检查车身焊缝有无开裂', '查看表面涂层是否剥落', '确认各连接螺栓紧固'],
    labelOffset: [0, 0.9, 0]
  },
  {
    partId: 'cabin',
    name: '驾驶室',
    info: '操作员工作区域，包含方向盘、操控杆和仪表盘。需确保操控装置灵敏可靠。',
    maintenance: ['检查方向盘自由行程', '测试各操控杆功能', '确认仪表盘指示正常', '检查座椅安全带'],
    labelOffset: [0, 1.8, -0.2]
  },
  {
    partId: 'forks',
    name: '货叉',
    info: '用于承载和搬运货物的核心部件。日常需检查货叉有无变形、裂纹和磨损。',
    maintenance: ['检查货叉有无弯曲变形', '查看叉尖磨损情况', '确认货叉定位销完好', '检查货叉挂架连接'],
    labelOffset: [0, 0.5, 1.6]
  },
  {
    partId: 'mast',
    name: '升降架',
    info: '支撑货叉升降的框架结构，需定期润滑和检查链条张力。',
    maintenance: ['检查升降链条张力', '对滑轨和链条加注润滑脂', '查看门架有无变形', '确认升降平稳无卡顿'],
    labelOffset: [0, 2.2, 0.95]
  },
  {
    partId: 'hydraulic',
    name: '液压系统',
    info: '驱动货叉升降和倾斜的动力系统。需定期检查油位、管路密封和油质。',
    maintenance: ['检查液压油油位', '查看管路有无渗漏', '确认油温正常', '定期更换液压油滤芯'],
    labelOffset: [0.25, 1.8, 0.85]
  },
  {
    partId: 'wheels',
    name: '车轮',
    info: '支撑整车行驶的部件，需检查胎压、磨损和轮毂螺栓。',
    maintenance: ['检查轮胎气压', '查看胎面磨损程度', '确认轮毂螺栓紧固', '检查转向轮间隙'],
    labelOffset: [0.9, 0.4, 0]
  },
  {
    partId: 'counterweight',
    name: '配重',
    info: '位于车尾的铸铁块，平衡货物重量防止前倾。需检查固定螺栓和有无裂纹。',
    maintenance: ['检查配重固定螺栓', '查看有无裂纹和损伤', '确认配重块无松动'],
    labelOffset: [0, 1.0, -1.1]
  },
  {
    partId: 'overheadGuard',
    name: '护顶架',
    info: '保护操作员免受坠落物伤害的安全装置。需检查结构完整性和连接紧固性。',
    maintenance: ['检查护顶架有无变形', '确认焊接点无开裂', '检查连接螺栓紧固', '清理顶部杂物'],
    labelOffset: [0, 2.5, -0.2]
  }
]

const INSPECTION_ORDER = ['forks', 'mast', 'hydraulic', 'wheels', 'cabin']

const DIAGNOSIS_SCENARIOS = [
  { partId: 'hydraulic', hint: '货叉升降缓慢且无力，液压油温度偏高', fault: '液压油不足或油泵磨损' },
  { partId: 'forks', hint: '货叉承载时出现明显弯曲，叉尖磨损严重', fault: '货叉变形超限需更换' },
  { partId: 'mast', hint: '升降过程中出现卡顿和异响', fault: '链条松弛或滑轨缺油' },
  { partId: 'wheels', hint: '行驶时方向盘抖动，轮胎偏磨', fault: '胎压不足或转向系统故障' },
  { partId: 'cabin', hint: '操控杆操作无响应，仪表盘报警灯亮', fault: '操控系统电路故障' }
]

const DIFFICULTY_CONFIG = {
  beginner: {
    label: '初级',
    timeLimit: 0,
    maxAttempts: 0,
    hintLevel: 'full',
    showHighlight: true,
    scoreMultiplier: 0.6
  },
  normal: {
    label: '中级',
    timeLimit: 180,
    maxAttempts: 3,
    hintLevel: 'partial',
    showHighlight: true,
    scoreMultiplier: 1.0
  },
  expert: {
    label: '高级',
    timeLimit: 90,
    maxAttempts: 1,
    hintLevel: 'none',
    showHighlight: false,
    scoreMultiplier: 1.5
  }
}

const geometryCache = new Map()

function getGeometry(key, factory) {
  if (!geometryCache.has(key)) {
    geometryCache.set(key, factory())
  }
  return geometryCache.get(key)
}

function getBoxGeometry(w, h, d) {
  return getGeometry(`box_${w}_${h}_${d}`, () => new THREE.BoxGeometry(w, h, d))
}

function getCylinderGeometry(rT, rB, h, seg) {
  return getGeometry(`cyl_${rT}_${rB}_${h}_${seg}`, () => new THREE.CylinderGeometry(rT, rB, h, seg))
}

export function clearGeometryCache() {
  geometryCache.forEach(geo => geo.dispose())
  geometryCache.clear()
}

function createMaterial(color, opts = {}) {
  return new THREE.MeshStandardMaterial({
    color: new THREE.Color(color),
    roughness: opts.roughness ?? 0.6,
    metalness: opts.metalness ?? 0.3,
    emissive: new THREE.Color(color),
    emissiveIntensity: 0,
    ...opts
  })
}

function createBox(w, h, d, material) {
  const mesh = new THREE.Mesh(getBoxGeometry(w, h, d), material)
  mesh.castShadow = true
  mesh.receiveShadow = true
  return mesh
}

function createCylinder(rT, rB, h, material, seg = 16) {
  const mesh = new THREE.Mesh(getCylinderGeometry(rT, rB, h, seg), material)
  mesh.castShadow = true
  mesh.receiveShadow = true
  return mesh
}

function createRoundedBox(w, h, d, r, material) {
  const shape = new THREE.Shape()
  const hw = w / 2 - r
  const hh = h / 2 - r
  shape.moveTo(-hw, -h / 2)
  shape.lineTo(hw, -h / 2)
  shape.quadraticCurveTo(w / 2, -h / 2, w / 2, -hh)
  shape.lineTo(w / 2, hh)
  shape.quadraticCurveTo(w / 2, h / 2, hw, h / 2)
  shape.lineTo(-hw, h / 2)
  shape.quadraticCurveTo(-w / 2, h / 2, -w / 2, hh)
  shape.lineTo(-w / 2, -hh)
  shape.quadraticCurveTo(-w / 2, -h / 2, -hw, -h / 2)

  const extrudeSettings = { depth: d, bevelEnabled: true, bevelThickness: r * 0.5, bevelSize: r * 0.5, bevelSegments: 3 }
  const key = `rbox_${w}_${h}_${d}_${r}`
  const geo = getGeometry(key, () => new THREE.ExtrudeGeometry(shape, extrudeSettings))
  const mesh = new THREE.Mesh(geo, material)
  mesh.castShadow = true
  mesh.receiveShadow = true
  return mesh
}

export function createWorkshop() {
  const group = new THREE.Group()

  const concreteMat = createMaterial('#6B7280', { roughness: 0.95, metalness: 0.05 })
  const ground = createBox(20, 0.05, 20, concreteMat)
  ground.position.set(0, -0.025, 0)
  ground.receiveShadow = true
  group.add(ground)

  const lineMat = createMaterial('#EAB308', { roughness: 0.7 })
  const linePositions = [
    { x: 0, z: 3, w: 8, d: 0.1 },
    { x: 0, z: -3, w: 8, d: 0.1 },
    { x: 3, z: 0, w: 0.1, d: 8 },
    { x: -3, z: 0, w: 0.1, d: 8 }
  ]
  linePositions.forEach(pos => {
    const line = createBox(pos.w, 0.01, pos.d, lineMat)
    line.position.set(pos.x, 0.01, pos.z)
    group.add(line)
  })

  const cautionMat = createMaterial('#F59E0B', { roughness: 0.6 })
  for (let i = -2; i <= 2; i++) {
    const stripe = createBox(0.6, 0.01, 0.12, i % 2 === 0 ? cautionMat : createMaterial('#1F2937'))
    stripe.position.set(i * 0.7, 0.012, 7)
    stripe.rotation.y = Math.PI / 6
    group.add(stripe)
    const stripe2 = createBox(0.6, 0.01, 0.12, i % 2 === 0 ? createMaterial('#1F2937') : cautionMat)
    stripe2.position.set(i * 0.7, 0.012, -7)
    stripe2.rotation.y = -Math.PI / 6
    group.add(stripe2)
  }

  const wallMat = createMaterial('#D1D5DB', { roughness: 0.85, metalness: 0.05 })
  const wallHeight = 4
  const wallThickness = 0.2
  const wallDist = 9.5

  const wallBack = createBox(20, wallHeight, wallThickness, wallMat)
  wallBack.position.set(0, wallHeight / 2, -wallDist)
  wallBack.receiveShadow = true
  group.add(wallBack)

  const wallLeft = createBox(wallThickness, wallHeight, 20, wallMat)
  wallLeft.position.set(-wallDist, wallHeight / 2, 0)
  wallLeft.receiveShadow = true
  group.add(wallLeft)

  const wallRight = createBox(wallThickness, wallHeight, 20, wallMat)
  wallRight.position.set(wallDist, wallHeight / 2, 0)
  wallRight.receiveShadow = true
  group.add(wallRight)

  const stripeMat = createMaterial('#3B82F6', { roughness: 0.5 })
  for (let i = 0; i < 4; i++) {
    const safetyStripe = createBox(20.5, 0.15, 0.05, stripeMat)
    safetyStripe.position.set(0, 0.5 + i * 1.0, -wallDist + 0.13)
    group.add(safetyStripe)
  }

  const signMat = createMaterial('#22C55E', { roughness: 0.4 })
  const signBg = createBox(1.4, 0.8, 0.03, signMat)
  signBg.position.set(0, 3.2, -wallDist + 0.12)
  group.add(signBg)
  const signBorder = createBox(1.5, 0.9, 0.02, createMaterial('#FFFFFF', { roughness: 0.3 }))
  signBorder.position.set(0, 3.2, -wallDist + 0.11)
  group.add(signBorder)

  const shelfMat = createMaterial('#6B7280', { roughness: 0.5, metalness: 0.4 })
  const shelfX = -8
  const shelfZ = -4

  for (let i = 0; i < 4; i++) {
    const shelf = createBox(2.0, 0.04, 0.6, shelfMat)
    shelf.position.set(shelfX, 0.6 + i * 0.7, shelfZ)
    group.add(shelf)

    const legL = createBox(0.04, 0.6 + i * 0.7, 0.04, shelfMat)
    legL.position.set(shelfX - 0.95, (0.6 + i * 0.7) / 2, shelfZ - 0.25)
    group.add(legL)

    const legR = createBox(0.04, 0.6 + i * 0.7, 0.04, shelfMat)
    legR.position.set(shelfX + 0.95, (0.6 + i * 0.7) / 2, shelfZ - 0.25)
    group.add(legR)

    const legBackL = createBox(0.04, 0.6 + i * 0.7, 0.04, shelfMat)
    legBackL.position.set(shelfX - 0.95, (0.6 + i * 0.7) / 2, shelfZ + 0.25)
    group.add(legBackL)

    const legBackR = createBox(0.04, 0.6 + i * 0.7, 0.04, shelfMat)
    legBackR.position.set(shelfX + 0.95, (0.6 + i * 0.7) / 2, shelfZ + 0.25)
    group.add(legBackR)
  }

  const toolRed = createMaterial('#DC2626', { roughness: 0.3, metalness: 0.6 })
  const toolBlue = createMaterial('#2563EB', { roughness: 0.3, metalness: 0.6 })
  const toolYellow = createMaterial('#D97706', { roughness: 0.3, metalness: 0.6 })

  const tool1 = createBox(0.06, 0.3, 0.03, toolRed)
  tool1.position.set(shelfX - 0.5, 0.8, shelfZ)
  group.add(tool1)
  const tool2 = createBox(0.06, 0.25, 0.03, toolBlue)
  tool2.position.set(shelfX + 0.3, 0.8, shelfZ)
  group.add(tool2)
  const tool3 = createBox(0.2, 0.03, 0.03, toolYellow)
  tool3.position.set(shelfX, 1.5, shelfZ)
  group.add(tool3)

  const barrelMat = createMaterial('#1F2937', { roughness: 0.4, metalness: 0.6 })
  const barrelBandMat = createMaterial('#6B7280', { roughness: 0.3, metalness: 0.7 })
  const barrelPositions = [
    { x: 7, z: -6 },
    { x: 7.7, z: -6 },
    { x: 7.35, z: -5.4 }
  ]
  barrelPositions.forEach(pos => {
    const barrel = createCylinder(0.32, 0.32, 0.9, barrelMat, 16)
    barrel.position.set(pos.x, 0.45, pos.z)
    group.add(barrel)
    const band1 = createCylinder(0.33, 0.33, 0.04, barrelBandMat, 16)
    band1.position.set(pos.x, 0.2, pos.z)
    group.add(band1)
    const band2 = createCylinder(0.33, 0.33, 0.04, barrelBandMat, 16)
    band2.position.set(pos.x, 0.7, pos.z)
    group.add(band2)
  })

  const palletMat = createMaterial('#92400E', { roughness: 0.9, metalness: 0.05 })
  const palletX = 6.5
  const palletZ = 3.5
  const palletBase = createBox(1.4, 0.08, 1.2, palletMat)
  palletBase.position.set(palletX, 0.04, palletZ)
  group.add(palletBase)
  for (let i = 0; i < 3; i++) {
    const slat = createBox(1.4, 0.06, 0.1, palletMat)
    slat.position.set(palletX, 0.12, palletZ - 0.4 + i * 0.4)
    group.add(slat)
  }
  const palletTop2 = createBox(1.4, 0.08, 1.2, palletMat)
  palletTop2.position.set(palletX, 0.5, palletZ)
  group.add(palletTop2)

  const boxMat = createMaterial('#D97706', { roughness: 0.7, metalness: 0.1 })
  const box1 = createBox(0.5, 0.4, 0.5, boxMat)
  box1.position.set(palletX - 0.3, 0.9, palletZ)
  group.add(box1)
  const box2 = createBox(0.4, 0.35, 0.4, createMaterial('#92400E', { roughness: 0.7, metalness: 0.1 }))
  box2.position.set(palletX + 0.2, 0.88, palletZ + 0.1)
  group.add(box2)

  const coneMat = createMaterial('#EF4444', { roughness: 0.5, metalness: 0.1 })
  const whiteMat = createMaterial('#FFFFFF', { roughness: 0.5 })
  const conePositions = [
    { x: -3, z: 6 }, { x: 3, z: 6 },
    { x: -3, z: -6 }, { x: 3, z: -6 }
  ]
  conePositions.forEach(pos => {
    const coneBase = createCylinder(0.18, 0.22, 0.05, coneMat, 12)
    coneBase.position.set(pos.x, 0.025, pos.z)
    group.add(coneBase)
    const coneBody = createCylinder(0.035, 0.16, 0.55, coneMat, 12)
    coneBody.position.set(pos.x, 0.325, pos.z)
    group.add(coneBody)
    const stripe = createCylinder(0.09, 0.09, 0.07, whiteMat, 12)
    stripe.position.set(pos.x, 0.38, pos.z)
    group.add(stripe)
    const stripe2 = createCylinder(0.13, 0.13, 0.05, whiteMat, 12)
    stripe2.position.set(pos.x, 0.2, pos.z)
    group.add(stripe2)
  })

  const lightMat = createMaterial('#F8FAFC', { roughness: 0.2, metalness: 0.1, emissive: new THREE.Color('#FFF8E1'), emissiveIntensity: 0.3 })
  for (let i = 0; i < 4; i++) {
    const lightFixture = createBox(0.6, 0.08, 0.6, lightMat)
    lightFixture.position.set(-6 + i * 4, wallHeight - 0.05, -4)
    group.add(lightFixture)
    const lightFixture2 = createBox(0.6, 0.08, 0.6, lightMat)
    lightFixture2.position.set(-6 + i * 4, wallHeight - 0.05, 4)
    group.add(lightFixture2)
  }

  const pipeMat = createMaterial('#9CA3AF', { roughness: 0.3, metalness: 0.7 })
  for (let i = 0; i < 3; i++) {
    const pipe = createCylinder(0.04, 0.04, 20, pipeMat, 8)
    pipe.rotation.z = Math.PI / 2
    pipe.position.set(0, 3.2 + i * 0.25, -wallDist + 0.15)
    group.add(pipe)
  }

  group.traverse(child => {
    if (child.isMesh) {
      child.castShadow = true
      child.receiveShadow = true
    }
  })

  return group
}

export function createForklift() {
  const group = new THREE.Group()
  const partMeshes = {}

  const bodyMat = createMaterial('#F5A623', { roughness: 0.4, metalness: 0.3 })
  const bodyDarkMat = createMaterial('#D4891A', { roughness: 0.5, metalness: 0.3 })
  const cabinMat = createMaterial('#4A5568', { roughness: 0.3, metalness: 0.4 })
  const cabinDarkMat = createMaterial('#2D3748', { roughness: 0.4, metalness: 0.4 })
  const forkMat = createMaterial('#A0AEC0', { roughness: 0.25, metalness: 0.7, polygonOffset: true, polygonOffsetFactor: -1, polygonOffsetUnits: -1 })
  const mastMat = createMaterial('#718096', { roughness: 0.3, metalness: 0.6, polygonOffset: true, polygonOffsetFactor: -1, polygonOffsetUnits: -1 })
  const hydraulicMat = createMaterial('#E8751A', { roughness: 0.3, metalness: 0.5, polygonOffset: true, polygonOffsetFactor: -1, polygonOffsetUnits: -1 })
  const wheelMat = createMaterial('#1F2937', { roughness: 0.8, metalness: 0.1 })
  const hubMat = createMaterial('#A0AEC0', { roughness: 0.2, metalness: 0.8 })
  const counterweightMat = createMaterial('#1A202C', { roughness: 0.6, metalness: 0.4 })
  const guardMat = createMaterial('#D69E2E', { roughness: 0.35, metalness: 0.5 })
  const windowMat = new THREE.MeshStandardMaterial({
    color: '#63B3ED',
    roughness: 0.05,
    metalness: 0.9,
    transparent: true,
    opacity: 0.4,
    emissive: new THREE.Color('#63B3ED'),
    emissiveIntensity: 0
  })
  const lightLensMat = new THREE.MeshStandardMaterial({
    color: '#FEE2E2',
    roughness: 0.1,
    metalness: 0.2,
    transparent: true,
    opacity: 0.8,
    emissive: new THREE.Color('#EF4444'),
    emissiveIntensity: 0.3
  })
  const headlightMat = new THREE.MeshStandardMaterial({
    color: '#FEF3C7',
    roughness: 0.1,
    metalness: 0.2,
    transparent: true,
    opacity: 0.9,
    emissive: new THREE.Color('#FDE68A'),
    emissiveIntensity: 0.5
  })

  // === 车身 ===
  const bodyGroup = new THREE.Group()
  bodyGroup.userData = { partId: 'body' }

  const bodyMain = createBox(1.05, 0.45, 1.5, bodyMat)
  bodyMain.position.set(0, 0.48, 0)
  bodyGroup.add(bodyMain)

  const bodyTop = createBox(1.1, 0.08, 1.55, bodyDarkMat)
  bodyTop.position.set(0, 0.74, 0)
  bodyGroup.add(bodyTop)

  const bodyBottom = createBox(1.15, 0.06, 1.6, bodyDarkMat)
  bodyBottom.position.set(0, 0.22, 0)
  bodyGroup.add(bodyBottom)

  const engineHood = createBox(0.95, 0.15, 0.5, bodyMat)
  engineHood.position.set(0, 0.58, 0.55)
  bodyGroup.add(engineHood)

  const hoodLine = createBox(0.02, 0.16, 0.52, bodyDarkMat)
  hoodLine.position.set(0, 0.58, 0.55)
  bodyGroup.add(hoodLine)

  const sidePanelL = createBox(0.03, 0.25, 0.8, bodyDarkMat)
  sidePanelL.position.set(-0.54, 0.42, 0.1)
  bodyGroup.add(sidePanelL)
  const sidePanelR = createBox(0.03, 0.25, 0.8, bodyDarkMat)
  sidePanelR.position.set(0.54, 0.42, 0.1)
  bodyGroup.add(sidePanelR)

  const frontLightL = createBox(0.12, 0.08, 0.03, headlightMat)
  frontLightL.position.set(-0.35, 0.55, 0.77)
  bodyGroup.add(frontLightL)
  const frontLightR = createBox(0.12, 0.08, 0.03, headlightMat)
  frontLightR.position.set(0.35, 0.55, 0.77)
  bodyGroup.add(frontLightR)

  const tailLightL = createBox(0.1, 0.06, 0.03, lightLensMat)
  tailLightL.position.set(-0.4, 0.55, -0.77)
  bodyGroup.add(tailLightL)
  const tailLightR = createBox(0.1, 0.06, 0.03, lightLensMat)
  tailLightR.position.set(0.4, 0.55, -0.77)
  bodyGroup.add(tailLightR)

  const stepL = createBox(0.25, 0.03, 0.3, bodyDarkMat)
  stepL.position.set(-0.55, 0.25, -0.15)
  bodyGroup.add(stepL)
  const stepR = createBox(0.25, 0.03, 0.3, bodyDarkMat)
  stepR.position.set(0.55, 0.25, -0.15)
  bodyGroup.add(stepR)

  const exhaust = createCylinder(0.03, 0.03, 0.35, createMaterial('#6B7280', { roughness: 0.3, metalness: 0.7 }), 8)
  exhaust.position.set(0.45, 0.65, -0.5)
  bodyGroup.add(exhaust)

  partMeshes.body = bodyGroup
  group.add(bodyGroup)

  // === 驾驶室 ===
  const cabinGroup = new THREE.Group()
  cabinGroup.userData = { partId: 'cabin' }

  const cabinBase = createBox(0.85, 0.08, 0.75, cabinDarkMat)
  cabinBase.position.set(0, 0.78, -0.2)
  cabinGroup.add(cabinBase)

  const cabinBack = createBox(0.85, 0.75, 0.06, cabinMat)
  cabinBack.position.set(0, 1.2, -0.55)
  cabinGroup.add(cabinBack)

  const cabinLeft = createBox(0.06, 0.75, 0.75, cabinMat)
  cabinLeft.position.set(-0.43, 1.2, -0.2)
  cabinGroup.add(cabinLeft)

  const cabinRight = createBox(0.06, 0.75, 0.75, cabinMat)
  cabinRight.position.set(0.43, 1.2, -0.2)
  cabinGroup.add(cabinRight)

  const frontWindowFrame = createBox(0.82, 0.5, 0.03, cabinMat)
  frontWindowFrame.position.set(0, 1.25, 0.16)
  cabinGroup.add(frontWindowFrame)
  const frontWindow = createBox(0.7, 0.38, 0.02, windowMat)
  frontWindow.position.set(0, 1.25, 0.18)
  cabinGroup.add(frontWindow)

  const sideWindowL = createBox(0.02, 0.35, 0.5, windowMat)
  sideWindowL.position.set(-0.44, 1.25, -0.2)
  cabinGroup.add(sideWindowL)
  const sideWindowR = createBox(0.02, 0.35, 0.5, windowMat)
  sideWindowR.position.set(0.44, 1.25, -0.2)
  cabinGroup.add(sideWindowR)

  const seat = createBox(0.35, 0.08, 0.35, createMaterial('#1F2937', { roughness: 0.8, metalness: 0.1 }))
  seat.position.set(0, 0.88, -0.25)
  cabinGroup.add(seat)
  const seatBack = createBox(0.35, 0.3, 0.06, createMaterial('#1F2937', { roughness: 0.8, metalness: 0.1 }))
  seatBack.position.set(0, 1.07, -0.4)
  cabinGroup.add(seatBack)

  const steeringCol = createCylinder(0.02, 0.02, 0.3, createMaterial('#374151', { roughness: 0.3, metalness: 0.6 }), 8)
  steeringCol.position.set(0, 1.05, 0.05)
  steeringCol.rotation.x = Math.PI / 6
  cabinGroup.add(steeringCol)
  const steeringWheel = createCylinder(0.14, 0.14, 0.02, createMaterial('#374151', { roughness: 0.4, metalness: 0.5 }), 20)
  steeringWheel.position.set(0, 1.2, 0.12)
  steeringWheel.rotation.x = Math.PI / 6
  cabinGroup.add(steeringWheel)

  const dashBoard = createBox(0.7, 0.15, 0.08, cabinDarkMat)
  dashBoard.position.set(0, 1.0, 0.12)
  cabinGroup.add(dashBoard)

  partMeshes.cabin = cabinGroup
  group.add(cabinGroup)

  // === 货叉 ===
  const forkGroup = new THREE.Group()
  forkGroup.userData = { partId: 'forks' }

  const forkBackrest = createBox(0.75, 0.55, 0.06, forkMat)
  forkBackrest.position.set(0, 0.45, 0.72)
  forkGroup.add(forkBackrest)

  const backrestTop = createBox(0.75, 0.04, 0.08, forkMat)
  backrestTop.position.set(0, 0.74, 0.73)
  forkGroup.add(backrestTop)

  const leftFork = createBox(0.1, 0.05, 1.0, forkMat)
  leftFork.position.set(-0.22, 0.19, 1.22)
  forkGroup.add(leftFork)
  const leftForkTip = createBox(0.1, 0.03, 0.08, forkMat)
  leftForkTip.position.set(-0.22, 0.17, 1.74)
  forkGroup.add(leftForkTip)

  const rightFork = createBox(0.1, 0.05, 1.0, forkMat)
  rightFork.position.set(0.22, 0.19, 1.22)
  forkGroup.add(rightFork)
  const rightForkTip = createBox(0.1, 0.03, 0.08, forkMat)
  rightForkTip.position.set(0.22, 0.17, 1.74)
  forkGroup.add(rightForkTip)

  const forkHook1 = createCylinder(0.03, 0.03, 0.08, forkMat, 8)
  forkHook1.position.set(-0.22, 0.45, 0.72)
  forkGroup.add(forkHook1)
  const forkHook2 = createCylinder(0.03, 0.03, 0.08, forkMat, 8)
  forkHook2.position.set(0.22, 0.45, 0.72)
  forkGroup.add(forkHook2)

  partMeshes.forks = forkGroup
  forkGroup.position.z = 0.10
  group.add(forkGroup)

  // === 升降架 ===
  const mastGroup = new THREE.Group()
  mastGroup.userData = { partId: 'mast' }

  const leftMastOuter = createBox(0.07, 1.6, 0.07, mastMat)
  leftMastOuter.position.set(-0.38, 1.1, 0.78)
  mastGroup.add(leftMastOuter)
  const rightMastOuter = createBox(0.07, 1.6, 0.07, mastMat)
  rightMastOuter.position.set(0.38, 1.1, 0.78)
  mastGroup.add(rightMastOuter)

  const leftMastInner = createBox(0.05, 1.4, 0.05, mastMat)
  leftMastInner.position.set(-0.38, 1.15, 0.84)
  mastGroup.add(leftMastInner)
  const rightMastInner = createBox(0.05, 1.4, 0.05, mastMat)
  rightMastInner.position.set(0.38, 1.15, 0.84)
  mastGroup.add(rightMastInner)

  const topBeam = createBox(0.83, 0.07, 0.07, mastMat)
  topBeam.position.set(0, 1.92, 0.78)
  mastGroup.add(topBeam)

  const midBeam = createBox(0.83, 0.05, 0.05, mastMat)
  midBeam.position.set(0, 1.2, 0.78)
  mastGroup.add(midBeam)

  const bottomBeam = createBox(0.83, 0.05, 0.05, mastMat)
  bottomBeam.position.set(0, 0.35, 0.78)
  mastGroup.add(bottomBeam)

  const chainL = createBox(0.015, 1.3, 0.015, createMaterial('#9CA3AF', { roughness: 0.2, metalness: 0.8 }))
  chainL.position.set(-0.32, 1.2, 0.83)
  mastGroup.add(chainL)
  const chainR = createBox(0.015, 1.3, 0.015, createMaterial('#9CA3AF', { roughness: 0.2, metalness: 0.8 }))
  chainR.position.set(0.32, 1.2, 0.83)
  mastGroup.add(chainR)

  const tiltCylL = createCylinder(0.025, 0.025, 0.6, mastMat, 8)
  tiltCylL.position.set(-0.42, 0.7, 0.6)
  tiltCylL.rotation.x = Math.PI / 8
  mastGroup.add(tiltCylL)
  const tiltCylR = createCylinder(0.025, 0.025, 0.6, mastMat, 8)
  tiltCylR.position.set(0.42, 0.7, 0.6)
  tiltCylR.rotation.x = Math.PI / 8
  mastGroup.add(tiltCylR)

  partMeshes.mast = mastGroup
  mastGroup.position.z = 0.05
  group.add(mastGroup)

  // === 液压系统 ===
  const hydraulicGroup = new THREE.Group()
  hydraulicGroup.userData = { partId: 'hydraulic' }

  const mainCylinder = createCylinder(0.045, 0.045, 1.3, hydraulicMat, 12)
  mainCylinder.position.set(0, 1.05, 0.68)
  hydraulicGroup.add(mainCylinder)

  const cylinderTop = createCylinder(0.055, 0.055, 0.06, createMaterial('#C4611A', { roughness: 0.3, metalness: 0.5 }), 12)
  cylinderTop.position.set(0, 1.72, 0.68)
  hydraulicGroup.add(cylinderTop)

  const cylinderBottom = createCylinder(0.06, 0.06, 0.08, createMaterial('#C4611A', { roughness: 0.3, metalness: 0.5 }), 12)
  cylinderBottom.position.set(0, 0.38, 0.68)
  hydraulicGroup.add(cylinderBottom)

  const pistonRod = createCylinder(0.02, 0.02, 0.5, createMaterial('#E2E8F0', { roughness: 0.1, metalness: 0.9 }), 8)
  pistonRod.position.set(0, 1.55, 0.68)
  hydraulicGroup.add(pistonRod)

  const hoseL = createCylinder(0.015, 0.015, 0.8, createMaterial('#1F2937', { roughness: 0.5, metalness: 0.3 }), 6)
  hoseL.position.set(-0.08, 0.8, 0.65)
  hydraulicGroup.add(hoseL)
  const hoseR = createCylinder(0.015, 0.015, 0.8, createMaterial('#1F2937', { roughness: 0.5, metalness: 0.3 }), 6)
  hoseR.position.set(0.08, 0.8, 0.65)
  hydraulicGroup.add(hoseR)

  const oilTank = createCylinder(0.12, 0.12, 0.2, createMaterial('#1F2937', { roughness: 0.4, metalness: 0.5 }), 10)
  oilTank.position.set(0.35, 0.45, -0.3)
  hydraulicGroup.add(oilTank)

  partMeshes.hydraulic = hydraulicGroup
  hydraulicGroup.position.z = 0.10
  group.add(hydraulicGroup)

  // === 车轮 ===
  const wheelGroup = new THREE.Group()
  wheelGroup.userData = { partId: 'wheels' }

  const wheelPositions = [
    { x: -0.6, z: 0.5, r: 0.22, steer: true },
    { x: 0.6, z: 0.5, r: 0.22, steer: true },
    { x: -0.6, z: -0.5, r: 0.25, steer: false },
    { x: 0.6, z: -0.5, r: 0.25, steer: false }
  ]
  wheelPositions.forEach(pos => {
    const tire = createCylinder(pos.r, pos.r, 0.18, wheelMat, 24)
    tire.rotation.z = Math.PI / 2
    tire.position.set(pos.x, pos.r, pos.z)
    wheelGroup.add(tire)

    const hub = createCylinder(pos.r * 0.4, pos.r * 0.4, 0.19, hubMat, 12)
    hub.rotation.z = Math.PI / 2
    hub.position.set(pos.x, pos.r, pos.z)
    wheelGroup.add(hub)

    const rim = createCylinder(pos.r * 0.65, pos.r * 0.65, 0.04, hubMat, 16)
    rim.rotation.z = Math.PI / 2
    rim.position.set(pos.x + 0.08, pos.r, pos.z)
    wheelGroup.add(rim)
    const rim2 = createCylinder(pos.r * 0.65, pos.r * 0.65, 0.04, hubMat, 16)
    rim2.rotation.z = Math.PI / 2
    rim2.position.set(pos.x - 0.08, pos.r, pos.z)
    wheelGroup.add(rim2)

    if (pos.steer) {
      const axle = createCylinder(0.03, 0.03, 0.2, createMaterial('#6B7280', { roughness: 0.3, metalness: 0.7 }), 8)
      axle.rotation.z = Math.PI / 2
      axle.position.set(pos.x, pos.r, pos.z)
      wheelGroup.add(axle)
    }
  })

  partMeshes.wheels = wheelGroup
  group.add(wheelGroup)

  // === 配重 ===
  const cwGroup = new THREE.Group()
  cwGroup.userData = { partId: 'counterweight' }

  const cwMain = createBox(0.95, 0.6, 0.4, counterweightMat)
  cwMain.position.set(0, 0.52, -0.9)
  cwGroup.add(cwMain)

  const cwTop = createBox(1.0, 0.08, 0.42, counterweightMat)
  cwTop.position.set(0, 0.86, -0.9)
  cwGroup.add(cwTop)

  const cwCurve = createCylinder(0.2, 0.2, 0.95, counterweightMat, 12)
  cwCurve.rotation.z = Math.PI / 2
  cwCurve.position.set(0, 0.52, -1.12)
  cwGroup.add(cwCurve)

  const hook = createCylinder(0.03, 0.03, 0.12, createMaterial('#6B7280', { roughness: 0.3, metalness: 0.7 }), 8)
  hook.position.set(0, 0.35, -1.12)
  cwGroup.add(hook)

  partMeshes.counterweight = cwGroup
  group.add(cwGroup)

  // === 护顶架 ===
  const guardGroup = new THREE.Group()
  guardGroup.userData = { partId: 'overheadGuard' }

  const postPositions = [
    [-0.4, 0, -0.52], [0.4, 0, -0.52],
    [-0.4, 0, 0.12], [0.4, 0, 0.12]
  ]
  postPositions.forEach(([px, , pz]) => {
    const post = createBox(0.05, 0.65, 0.05, guardMat)
    post.position.set(px, 1.72, pz)
    guardGroup.add(post)

    const postBase = createBox(0.08, 0.04, 0.08, guardMat)
    postBase.position.set(px, 1.38, pz)
    guardGroup.add(postBase)
  })

  const roofTop = createBox(0.88, 0.05, 0.72, guardMat)
  roofTop.position.set(0, 2.06, -0.2)
  guardGroup.add(roofTop)

  const roofFront = createBox(0.88, 0.04, 0.05, guardMat)
  roofFront.position.set(0, 2.04, 0.14)
  guardGroup.add(roofFront)

  const roofBack = createBox(0.88, 0.04, 0.05, guardMat)
  roofBack.position.set(0, 2.04, -0.54)
  guardGroup.add(roofBack)

  const crossBar1 = createBox(0.05, 0.04, 0.72, guardMat)
  crossBar1.position.set(-0.2, 2.04, -0.2)
  guardGroup.add(crossBar1)
  const crossBar2 = createBox(0.05, 0.04, 0.72, guardMat)
  crossBar2.position.set(0.2, 2.04, -0.2)
  guardGroup.add(crossBar2)

  partMeshes.overheadGuard = guardGroup
  group.add(guardGroup)

  group.traverse(child => {
    if (child.isMesh) {
      child.castShadow = true
      child.receiveShadow = true
    }
  })

  return { group, partMeshes }
}

export function getPartsConfig() {
  return PARTS_CONFIG
}

export function getPartInfo(partId) {
  return PARTS_CONFIG.find(p => p.partId === partId) || null
}

export function getInspectionOrder() {
  return INSPECTION_ORDER
}

export function getDiagnosisScenarios() {
  return DIAGNOSIS_SCENARIOS
}

export function getDifficultyConfig() {
  return DIFFICULTY_CONFIG
}

export function getDifficultySettings(level) {
  return DIFFICULTY_CONFIG[level] || DIFFICULTY_CONFIG.normal
}

export function highlightPart(partMesh, color = '#00ff88', intensity = 0.4) {
  const targetColor = new THREE.Color(color)
  if (partMesh.isMesh) {
    if (partMesh.material && partMesh.material.emissive) {
      partMesh.material.emissive.copy(targetColor)
      partMesh.material.emissiveIntensity = intensity
    }
  } else if (partMesh.isGroup) {
    partMesh.traverse(child => {
      if (child.isMesh && child.material && child.material.emissive) {
        child.material.emissive.copy(targetColor)
        child.material.emissiveIntensity = intensity
      }
    })
  }
}

export function unhighlightPart(partMesh) {
  if (partMesh.isMesh) {
    if (partMesh.material && partMesh.material.emissive) {
      partMesh.material.emissive.copy(partMesh.material.color)
      partMesh.material.emissiveIntensity = 0
    }
  } else if (partMesh.isGroup) {
    partMesh.traverse(child => {
      if (child.isMesh && child.material && child.material.emissive) {
        child.material.emissive.copy(child.material.color)
        child.material.emissiveIntensity = 0
      }
    })
  }
}

export function flashPart(partMesh, time, color = '#ff0000') {
  const intensity = (Math.sin(time * 3) + 1) / 2 * 0.4 + 0.2
  highlightPart(partMesh, color, intensity)
}

export function checkWebGL() {
  try {
    const canvas = document.createElement('canvas')
    return !!(
      window.WebGLRenderingContext &&
      (canvas.getContext('webgl') || canvas.getContext('experimental-webgl'))
    )
  } catch (e) {
    return false
  }
}

export function createPartLabels() {
  const labels = {}
  PARTS_CONFIG.forEach(part => {
    const div = document.createElement('div')
    div.className = 'part-label'
    div.textContent = part.name
    div.style.cssText = `
      padding: 2px 8px;
      background: rgba(0, 0, 0, 0.65);
      color: #fff;
      font-size: 11px;
      border-radius: 4px;
      pointer-events: none;
      white-space: nowrap;
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
      opacity: 0;
      transition: opacity 0.2s ease;
    `
    const label = new CSS2DObject(div)
    const [x, y, z] = part.labelOffset || [0, 0, 0]
    label.position.set(x, y, z)
    label.userData = { partId: part.partId }
    labels[part.partId] = label
  })
  return labels
}

export function showLabel(labels, partId) {
  if (labels[partId]) {
    labels[partId].element.style.opacity = '1'
  }
}

export function hideLabel(labels, partId) {
  if (labels[partId]) {
    labels[partId].element.style.opacity = '0'
  }
}

export function hideAllLabels(labels) {
  Object.values(labels).forEach(label => {
    label.element.style.opacity = '0'
  })
}
