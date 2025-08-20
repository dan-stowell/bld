# mount local ssd
sudo mkfs.ext4 -F /dev/nvme0n1
sudo mkdir -p /mnt/disks/local-ssd
sudo mount -o discard,defaults /dev/nvme0n1 /mnt/disks/local-ssd
df -h | grep /mnt/disks/local-ssd

# create ssh key for github
ssh-keygen -t ed25519 -C "dstowell@gmail.com"
eval "$(ssh-agent -s)"
ssh-add ~/.ssh/id_ed25519
cat ~/.ssh/id_ed25519.pub 

sudo apt-get install git

wget https://github.com/bazelbuild/bazelisk/releases/download/v1.27.0/bazelisk-linux-amd64
sudo apt-get install npm
sudo npm install -g @anthropic-ai/claude-code
