@C:\Windows\System32\OpenSSH\ssh.exe -i C:\Users\ZHENG\.ssh\pve-04-colleague -o StrictHostKeyChecking=no -o ConnectTimeout=10 -p 2204 root@183.36.195.104 "pgrep -f selective_caption.py" > E:\FL\training-app\叉车维修培训学员端跨端应用\ssh_out.txt 2>&1
@echo PIDCHECK_DONE >> E:\FL\training-app\叉车维修培训学员端跨端应用\ssh_out.txt
@C:\Windows\System32\OpenSSH\ssh.exe -i C:\Users\ZHENG\.ssh\pve-04-colleague -o StrictHostKeyChecking=no -o ConnectTimeout=10 -p 2204 root@183.36.195.104 "tail -3 /root/selective_caption.log 2>/dev/null" >> E:\FL\training-app\叉车维修培训学员端跨端应用\ssh_out.txt 2>&1
@echo LOGCHECK_DONE >> E:\FL\training-app\叉车维修培训学员端跨端应用\ssh_out.txt
@C:\Windows\System32\OpenSSH\ssh.exe -i C:\Users\ZHENG\.ssh\pve-04-colleague -o StrictHostKeyChecking=no -o ConnectTimeout=10 -p 2204 root@183.36.195.104 "cat /root/.caption_high_done 2>/dev/null" >> E:\FL\training-app\叉车维修培训学员端跨端应用\ssh_out.txt 2>&1
@echo DONECHECK_DONE >> E:\FL\training-app\叉车维修培训学员端跨端应用\ssh_out.txt
@C:\Windows\System32\OpenSSH\ssh.exe -i C:\Users\ZHENG\.ssh\pve-04-colleague -o StrictHostKeyChecking=no -o ConnectTimeout=10 -p 2204 root@183.36.195.104 "python3 -c 'import json; d=json.load(open(\"/root/.caption_cache.json\")); print(f\"{len(d)} 条图注缓存\")' 2>/dev/null" >> E:\FL\training-app\叉车维修培训学员端跨端应用\ssh_out.txt 2>&1
@echo CACHECHECK_DONE >> E:\FL\training-app\叉车维修培训学员端跨端应用\ssh_out.txt
@C:\Windows\System32\OpenSSH\ssh.exe -i C:\Users\ZHENG\.ssh\pve-04-colleague -o StrictHostKeyChecking=no -o ConnectTimeout=10 -p 2204 root@183.36.195.104 "tail -3 /root/after_mineru2kb.log 2>/dev/null" >> E:\FL\training-app\叉车维修培训学员端跨端应用\ssh_out.txt 2>&1
@echo INGESTCHECK_DONE >> E:\FL\training-app\叉车维修培训学员端跨端应用\ssh_out.txt