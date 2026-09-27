$ErrorActionPreference = 'Continue'
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
& 'D:\软件\HBuilderX.5.23.2026080626\HBuilderX\cli.exe' launch app-android --pagePath pages/courses/course-detail --project D:\FL\training-app\叉车维修培训学员端跨端应用 --pageQuery id=1 --deviceId 192.168.10.54:43211 *> 'D:\FL\.ci-verify\1331\before\raw\launch-nav-7035608617274a45a79273d4358c823c.out'
