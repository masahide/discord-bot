# Welcome to your CDK TypeScript project!

### 必要ソフト

- go version go1.17 linux/amd64
- node v16.13.1 (npm v8.3.0)
- cdk 2.2.0 (build 4f5c27c)

```bash
# user名, パラメータストアのkey名, slack workflow builder webhook URLを設定
SVNAME=pve20
export SSMPATH=/lambda/discordbot/$SVNAME

# 対象リージョン指定
export AWS_DEFAULT_REGION=ap-northeast-1
export CDK_DEFAULT_REGION=$AWS_DEFAULT_REGION
# 対象アカウントID
export CDK_DEFAULT_ACCOUNT=xxxxxxxxx


# SSMパラメータストアに登録
# ( パスワード更新時など、上書きする場合は`--overwrite`オプションを追加して実行する)
aws ssm put-parameter --type 'SecureString' --name $SSMPATH/slackurl --value $SLACKURL
aws ssm put-parameter --type 'SecureString' --name $SSMPATH/caldavpass --value $CALDAVPASS


# build
npm run build

# cdk 初期設定
cdk bootstrap
# diff確認
cdk diff

# デプロイ
cdk deploy

# デプロイした環境を削除する場合は以下
cdk destroy
```

This is a blank project for TypeScript development with CDK.

The `cdk.json` file tells the CDK Toolkit how to execute your app.

## Useful commands

 * `npm run build`   compile typescript to js
 * `npm run watch`   watch for changes and compile
 * `npm run test`    perform the jest unit tests
 * `cdk deploy`      deploy this stack to your default AWS account/region
 * `cdk diff`        compare deployed stack with current state
 * `cdk synth`       emits the synthesized CloudFormation template
