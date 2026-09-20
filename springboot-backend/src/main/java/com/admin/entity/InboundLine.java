package com.admin.entity;

import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableId;
import lombok.Data;

import java.io.Serializable;

/**
 * <p>
 * 线路 = 车友 × 机器 × 落地组。
 * landing_id 空=该机器的直连线路;非空=该落地的中转线路。
 * 同机的直连和每个中转各算一条线路:各一条订阅、各一份流量/到期配额。
 * 已用流量不存这里,实时汇总该线路各协议对应转发的 in_flow+out_flow。
 * </p>
 *
 * @author QAQ
 * @since 2026-07-26
 */
@Data
public class InboundLine implements Serializable {

    private static final long serialVersionUID = 1L;

    @TableId(value = "id", type = IdType.AUTO)
    private Long id;

    /** 车友 */
    private Long userId;

    /** 机器 */
    private Long nodeId;

    /** 落地ID:空=直连线路 */
    private Long landingId;

    /** 该线路的订阅 token */
    private String subToken;

    /** 该线路流量配额(GB);0/null=不单独限,只受账号总流量约束 */
    private Long flow;

    /** 该线路到期时间(epoch ms);空=不单独限 */
    private Long expTime;

    /** 1=正常 0=已停(超额/到期) */
    private Integer status;

    private Long createdTime;

    private Long updatedTime;
}
