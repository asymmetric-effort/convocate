# svr00 Repository Cutover Runbook

## Overview
This document describes the big-bang cutover from the svr00 repository
to the convocate repository for infrastructure management.

## Pre-cutover Checklist
- [ ] All Ansible roles from svr00 replicated in convocate/infrastructure/
- [ ] Cluster A provisioned and tested from convocate repo
- [ ] Cluster B provisioned and tested from convocate repo  
- [ ] cloudflared tunnel working from convocate repo
- [ ] All PDV tests pass on both clusters
- [ ] Self-hosted runner operational
- [ ] CD pipeline functional

## Cutover Steps

### 1. Freeze svr00
- Stop any cron jobs on 192.168.3.159 that reference svr00
- Verify no running processes depend on svr00 repo
- `ssh $HYPERVISOR_USER@$CLUSTER_B_HOST "crontab -l"` — remove svr00 entries

### 2. Provision Cluster B from convocate
```bash
# From the self-hosted runner (192.168.3.90):
ansible-playbook infrastructure/playbooks/destroy.yml -i inventory/cluster-b.yml
ansible-playbook infrastructure/playbooks/provision.yml -i inventory/cluster-b.yml
ansible-playbook infrastructure/playbooks/cluster.yml -i inventory/cluster-b.yml
ansible-playbook infrastructure/playbooks/harden.yml -i inventory/cluster-b.yml
ansible-playbook infrastructure/playbooks/verify.yml -i inventory/cluster-b.yml
ansible-playbook infrastructure/playbooks/openbao-bootstrap.yml -i inventory/cluster-b.yml
ansible-playbook infrastructure/playbooks/deploy.yml -i inventory/cluster-b.yml
```

### 3. Verify Cluster B
- Run full PDV suite against Cluster B
- Verify ZTNA access (cloudflared)
- Verify Grafana accessible at grafana.$DNS_DOMAIN
- Verify Convocate accessible at app.convocate.$DNS_DOMAIN
- Verify OpenBao accessible at auth.$DNS_DOMAIN
- Users can log in with MFA

### 4. Archive svr00
```bash
# On GitHub:
# Settings → Danger Zone → Archive this repository
```
- Archive svr00 on GitHub (read-only)
- Optionally remove the clone from 192.168.3.159

### 5. Post-cutover Verification
- Verify nightly Cluster A recycle runs (next 00:00 CT)
- Verify CD pipeline deploys on next merge to main
- Monitor for 24 hours

## Rollback Plan
If cutover fails:
1. Unarchive svr00 on GitHub
2. On 192.168.3.159: `cd ~/git/svr00 && vagrant up && ansible-playbook ansible/site.yml`
3. Restore cron jobs

## Timeline
- Cutover window: off-hours (recommend Saturday morning)
- Expected duration: 2-4 hours
- Monitoring period: 24 hours post-cutover
