module.exports = {
  globalTeardown: '@dcloudio/uni-automator/dist/teardown.js',
  testEnvironment: '@dcloudio/uni-automator/dist/environment.js',
  testEnvironmentOptions: {
    compile: true,
    h5: {
      url: "http://localhost:5173/h5/",
      options: {
        headless: true
      }
    },
    "app-plus": {
      android: {
        appid: "",
        package: "",
        executablePath: "HBuilderX/plugins/launcher/base/android_base.apk"
      },
      ios: {
        id: "",
        executablePath: "HBuilderX/plugins/launcher/base/Pandora_simulator.app"
      }
    },
    "mp-weixin": {
      port: 9420,
      account: "",
      args: "",
      cwd: "",
      launch: true,
      teardown: "disconnect",
      remote: false,
      executablePath: ""
    }
  },
  testTimeout: 15000,
  reporters: ['default'],
  watchPathIgnorePatterns: ['/node_modules/', '/dist/', '/.git/'],
  moduleFileExtensions: ['js', 'json'],
  rootDir: __dirname,
  testMatch: ['<rootDir>/pages/**/*.test.[jt]s?(x)'],
  testPathIgnorePatterns: ['/node_modules/']
}