package com.admin.common.dto;

import lombok.Data;

import javax.validation.constraints.Max;
import javax.validation.constraints.Min;
import javax.validation.constraints.NotNull;
import java.util.List;

/**
 * 给某入站分配一个子账号(车友),带限速/到期/流量配额。
 */
@Data
public class InboundUserDto {

    /** 单入站分配时用(assign);机器卡整机分配(assign-all)可不传 */
    private Long inboundId;

    @NotNull(message = "用户不能为空")
    private Long userId;

    /** 机器卡整机分配:只分配该节点(机器)上的所有协议;不传=所有节点 */
    private Long nodeId;

    /** 分配的是不是中转组:true=分该机某落地的中转协议(配 landingId);false/空=分该机的直连协议 */
    private Boolean relay;

    /** 中转分配时指定落地(relay=true 时用);直连分配不用 */
    private Long landingId;

    /** 限速规则ID(可空=不限速) */
    private Integer speedId;

    /** 到期时间 epoch ms(可空=永不过期) */
    private Long expTime;

    /** 流量配额,单位字节(可空=不改);写到该用户 User.flow */
    private Long flow;

    /** 协议级分配使用的公网 gost 端口起始值;空=自动分配,不落库 */
    @Min(value = 1, message = "公网端口必须在1-65535范围内")
    @Max(value = 65535, message = "公网端口必须在1-65535范围内")
    private Integer publicPort;

    /** 协议级分配(assign-all):只分配这些入站id(须同属 nodeId+relay+landingId 限定的同一条线路);
     *  空=该线路全部协议。线路额度/到期仍按 flow/expTime 写到 InboundLine,与整条分配一致。 */
    private List<Long> inboundIds;
}
