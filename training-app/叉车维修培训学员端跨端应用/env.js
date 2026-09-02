// 测试环境配置
module.exports = {
  // H5 平台配置
  h5: {
    url: 'http://localhost:5173/h5/',
    options: {
      headless: true
    }
  },
  // Android 平台配置
  android: {
    appid: '__UNI__1C1D180',
    package: 'com.example.forklifttraining',
    executablePath: 'HBuilderX/plugins/launcher/base/android_base.apk'
  },
  // iOS 平台配置
  ios: {
    id: '',
    executablePath: 'HBuilderX/plugins/launcher/base/Pandora_simulator.app'
  },
  // 微信小程序平台配置
  'mp-weixin': {
    port: 9420,
    account: '',
    args: '',
    cwd: '',
    launch: true,
    teardown: 'disconnect',
    remote: false,
    executablePath: ''
  }
};